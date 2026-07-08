package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func (cfg *RoutingCfg) UnmarshalJSON(data []byte) error {
	type alias RoutingCfg
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	allowed := map[string]bool{
		"enabled":           true,
		"defaultTier":       true,
		"default_tier":      true,
		"taskTiers":         true,
		"task_tiers":        true,
		"tiers":             true,
		"budgetsUSD":        true,
		"budgets_usd":       true,
		"maxTokens":         true,
		"max_tokens":        true,
		"costPerMTokIn":     true,
		"cost_per_mtok_in":  true,
		"costPerMTokOut":    true,
		"cost_per_mtok_out": true,
	}
	for key := range raw {
		if !allowed[key] {
			return fmt.Errorf("routing.%s: unknown field", key)
		}
	}
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*cfg = RoutingCfg(decoded)
	return nil
}

func (tier *TierCfg) UnmarshalJSON(data []byte) error {
	type alias TierCfg
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	allowed := map[string]bool{
		"model":             true,
		"maxTokens":         true,
		"max_tokens":        true,
		"budgetUSD":         true,
		"budget_usd":        true,
		"costPerMTokIn":     true,
		"cost_per_mtok_in":  true,
		"costPerMTokOut":    true,
		"cost_per_mtok_out": true,
	}
	for key := range raw {
		if !allowed[key] {
			return fmt.Errorf("routing.tiers.%s: unknown field", key)
		}
	}
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*tier = TierCfg(decoded)
	return nil
}

// RoutingCfg is the deterministic model-tier policy stored in config.json.
type RoutingCfg struct {
	Enabled        bool               `json:"enabled"`
	DefaultTier    string             `json:"defaultTier,omitempty"`
	TaskTiers      map[string]string  `json:"taskTiers,omitempty"`
	Tiers          map[string]TierCfg `json:"tiers,omitempty"`
	BudgetsUSD     map[string]string  `json:"budgetsUSD,omitempty"`
	MaxTokens      map[string]int64   `json:"maxTokens,omitempty"`
	CostPerMTokIn  map[string]string  `json:"costPerMTokIn,omitempty"`
	CostPerMTokOut map[string]string  `json:"costPerMTokOut,omitempty"`
}

// TierCfg describes a named routing tier without binding specd to a host model.
type TierCfg struct {
	Model          string `json:"model,omitempty"`
	MaxTokens      int64  `json:"maxTokens,omitempty"`
	BudgetUSD      string `json:"budgetUSD,omitempty"`
	CostPerMTokIn  string `json:"costPerMTokIn,omitempty"`
	CostPerMTokOut string `json:"costPerMTokOut,omitempty"`
}

// RoutingState is additive state.json metadata. It is derived from local
// config/task state and never from a model or network call.
type RoutingState struct {
	Tasks     map[string]TaskRoutingState `json:"tasks,omitempty"`
	Economics RoutingEconomics            `json:"economics,omitempty"`
}

// TaskRoutingState records the deterministic tier decision for a task.
type TaskRoutingState struct {
	Tier       string `json:"tier"`
	Model      string `json:"model,omitempty"`
	Reason     string `json:"reason,omitempty"`
	BudgetUSD  string `json:"budgetUSD,omitempty"`
	MaxTokens  int64  `json:"maxTokens,omitempty"`
	CostUSD    string `json:"costUSD,omitempty"`
	Tokens     int64  `json:"tokens,omitempty"`
	BrakeLevel string `json:"brakeLevel,omitempty"`
}

// RoutingEconomics is a stable cost rollup for reports.
type RoutingEconomics struct {
	TotalCostUSD string                        `json:"totalCostUSD,omitempty"`
	TotalTokens  int64                         `json:"totalTokens,omitempty"`
	ByTier       map[string]RoutingTierEconomy `json:"byTier,omitempty"`
}

type RoutingTierEconomy struct {
	CostUSD string `json:"costUSD,omitempty"`
	Tokens  int64  `json:"tokens,omitempty"`
	Tasks   int    `json:"tasks,omitempty"`
}

type routingMoney int64

const routingMoneyScale routingMoney = 1_000_000

// ResolveRoutingState builds the routing state from config and current task
// telemetry. The policy is deterministic: explicit task tier wins, otherwise
// the configured default tier, otherwise the lexicographically first tier.
func ResolveRoutingState(cfg RoutingCfg, tasks map[string]TaskState) (RoutingState, error) {
	if !cfg.Enabled {
		return RoutingState{}, nil
	}
	if err := ValidateRoutingConfig(&cfg); err != nil {
		return RoutingState{}, err
	}
	state := RoutingState{
		Tasks:     map[string]TaskRoutingState{},
		Economics: RoutingEconomics{ByTier: map[string]RoutingTierEconomy{}},
	}
	var total routingMoney
	var totalTokens int64
	ids := make([]string, 0, len(tasks))
	for id := range tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		rec := tasks[id]
		tierName := cfg.TaskTiers[id]
		reason := "task override"
		if tierName == "" {
			tierName = cfg.DefaultTier
			reason = "default tier"
		}
		tier := cfg.Tiers[tierName]
		tokens := taskTokens(rec)
		cost, err := taskCost(rec, tier)
		if err != nil {
			return RoutingState{}, fmt.Errorf("routing.%s: %w", id, err)
		}
		budget := tier.BudgetUSD
		if budget == "" {
			budget = cfg.BudgetsUSD[tierName]
		}
		limit, err := parseRoutingMoneyAllowEmpty(budget)
		if err != nil {
			return RoutingState{}, fmt.Errorf("routing.%s.budgetUSD: %w", tierName, err)
		}
		brake := string(CostBrakeNone)
		if limit > 0 {
			brake = string(EvaluateCostBrake(float64(cost)/float64(routingMoneyScale), float64(limit)/float64(routingMoneyScale)))
		}
		state.Tasks[id] = TaskRoutingState{
			Tier:       tierName,
			Model:      tier.Model,
			Reason:     reason,
			BudgetUSD:  budget,
			MaxTokens:  tierMaxTokens(cfg, tierName, tier),
			CostUSD:    formatRoutingMoney(cost),
			Tokens:     tokens,
			BrakeLevel: brake,
		}
		total += cost
		totalTokens += tokens
		eco := state.Economics.ByTier[tierName]
		eco.Tasks++
		eco.Tokens += tokens
		eco.CostUSD = formatRoutingMoney(parseRoutingMoneyMust(eco.CostUSD) + cost)
		state.Economics.ByTier[tierName] = eco
	}
	state.Economics.TotalCostUSD = formatRoutingMoney(total)
	state.Economics.TotalTokens = totalTokens
	return state, nil
}

func ResolveRoutingStamps(cfg RoutingCfg, tasks map[string]TaskState) (map[string]RoutingStamp, RoutingEconomics, error) {
	resolved, err := ResolveRoutingState(cfg, tasks)
	if err != nil || len(resolved.Tasks) == 0 {
		return nil, RoutingEconomics{}, err
	}
	stamps := map[string]RoutingStamp{}
	for id, task := range resolved.Tasks {
		ruleIndex := 1
		if task.Reason == "task override" {
			ruleIndex = 0
		}
		budget, err := parseRoutingMoneyAllowEmpty(task.BudgetUSD)
		if err != nil {
			return nil, RoutingEconomics{}, err
		}
		stamps[id] = RoutingStamp{
			Tier:      task.Tier,
			BudgetUSD: float64(budget) / float64(routingMoneyScale),
			RuleIndex: ruleIndex,
		}
	}
	return stamps, resolved.Economics, nil
}

func RoutingEconomicsFromState(state *State) RoutingEconomics {
	if state == nil {
		return RoutingEconomics{}
	}
	economics := RoutingEconomics{ByTier: map[string]RoutingTierEconomy{}}
	var total routingMoney
	var totalTokens int64
	ids := make([]string, 0, len(state.Tasks))
	for id := range state.Tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		stamp, ok := state.Routing[id]
		if !ok {
			continue
		}
		rec := state.Tasks[id]
		tokens := taskTokens(rec)
		cost, _ := taskCost(rec, TierCfg{})
		eco := economics.ByTier[stamp.Tier]
		eco.Tasks++
		eco.Tokens += tokens
		eco.CostUSD = formatRoutingMoney(parseRoutingMoneyMust(eco.CostUSD) + cost)
		economics.ByTier[stamp.Tier] = eco
		total += cost
		totalTokens += tokens
	}
	economics.TotalCostUSD = formatRoutingMoney(total)
	economics.TotalTokens = totalTokens
	return economics
}

func ValidateRoutingConfig(cfg *RoutingCfg) error {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	if len(cfg.Tiers) == 0 {
		return fmt.Errorf("routing.tiers required when routing.enabled=true")
	}
	if cfg.DefaultTier == "" {
		names := make([]string, 0, len(cfg.Tiers))
		for name := range cfg.Tiers {
			names = append(names, name)
		}
		sort.Strings(names)
		cfg.DefaultTier = names[0]
	}
	if _, ok := cfg.Tiers[cfg.DefaultTier]; !ok {
		return fmt.Errorf("routing.defaultTier %q is not defined", cfg.DefaultTier)
	}
	for task, tier := range cfg.TaskTiers {
		if task == "" {
			return fmt.Errorf("routing.taskTiers contains empty task id")
		}
		if _, ok := cfg.Tiers[tier]; !ok {
			return fmt.Errorf("routing.taskTiers.%s references undefined tier %q", task, tier)
		}
	}
	for name, tier := range cfg.Tiers {
		if name == "" {
			return fmt.Errorf("routing.tiers contains empty tier")
		}
		if tier.MaxTokens < 0 {
			return fmt.Errorf("routing.tiers.%s.maxTokens must be non-negative", name)
		}
		if _, err := parseRoutingMoneyAllowEmpty(tier.BudgetUSD); err != nil {
			return fmt.Errorf("routing.tiers.%s.budgetUSD: %w", name, err)
		}
		if _, err := parseRoutingMoneyAllowEmpty(tier.CostPerMTokIn); err != nil {
			return fmt.Errorf("routing.tiers.%s.costPerMTokIn: %w", name, err)
		}
		if _, err := parseRoutingMoneyAllowEmpty(tier.CostPerMTokOut); err != nil {
			return fmt.Errorf("routing.tiers.%s.costPerMTokOut: %w", name, err)
		}
	}
	return nil
}

func taskTokens(rec TaskState) int64 {
	if rec.Telemetry == nil {
		return 0
	}
	return int64(rec.Telemetry.Tokens)
}

func taskCost(rec TaskState, tier TierCfg) (routingMoney, error) {
	if rec.Telemetry != nil && rec.Telemetry.Cost != "" {
		return parseRoutingMoneyAllowEmpty(rec.Telemetry.Cost)
	}
	tokens := taskTokens(rec)
	if tokens == 0 {
		return 0, nil
	}
	rate, err := parseRoutingMoneyAllowEmpty(tier.CostPerMTokIn)
	if err != nil {
		return 0, err
	}
	if rate == 0 {
		return 0, nil
	}
	return routingMoney((int64(rate) * tokens) / 1_000_000), nil
}

func tierMaxTokens(cfg RoutingCfg, name string, tier TierCfg) int64 {
	if tier.MaxTokens > 0 {
		return tier.MaxTokens
	}
	return cfg.MaxTokens[name]
}

func parseRoutingMoneyMust(s string) routingMoney {
	v, _ := parseRoutingMoneyAllowEmpty(s)
	return v
}

func parseRoutingMoneyAllowEmpty(s string) (routingMoney, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	neg := strings.HasPrefix(s, "-")
	if neg {
		return 0, fmt.Errorf("must be non-negative")
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid decimal %q", s)
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid decimal %q", s)
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
		if len(frac) > 6 {
			frac = frac[:6]
		}
	}
	for len(frac) < 6 {
		frac += "0"
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid decimal %q", s)
	}
	return routingMoney(whole)*routingMoneyScale + routingMoney(f), nil
}

func formatRoutingMoney(v routingMoney) string {
	whole := int64(v / routingMoneyScale)
	frac := int64(v % routingMoneyScale)
	if frac == 0 {
		return strconv.FormatInt(whole, 10)
	}
	s := fmt.Sprintf("%d.%06d", whole, frac)
	return strings.TrimRight(s, "0")
}
