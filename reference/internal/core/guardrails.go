package core

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	guardrailsFileName = "guardrails.json"
	guardrailsMaxSize  = 1 << 20
)

type GuardrailsConfig struct {
	BaseRef           string          `json:"baseRef,omitempty"`
	ForbiddenImports  []GuardrailRule `json:"forbiddenImports,omitempty"`
	ForbiddenPatterns []GuardrailRule `json:"forbiddenPatterns,omitempty"`
	ForbiddenPaths    []GuardrailRule `json:"forbiddenPaths,omitempty"`
	ForbiddenCommands []GuardrailRule `json:"forbiddenCommands,omitempty"`
}

type GuardrailRule struct {
	ID      string `json:"id,omitempty"`
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}

type compiledGuardrailRule struct {
	rule    GuardrailRule
	pattern *regexp.Regexp
	path    *regexp.Regexp
}

type GuardrailsResult struct {
	Present  bool
	Warnings []Violation
}

func GuardrailsPath(root string) string {
	return filepath.Join(root, ".specd", guardrailsFileName)
}

func DefaultGuardrailsJSON() string {
	return `{
  "baseRef": "HEAD",
  "forbiddenImports": [
    {
      "id": "no-weak-crypto-md5",
      "path": "\\.go$",
      "pattern": "\"crypto/md5\"",
      "message": "use a collision-resistant hash for security-sensitive code"
    },
    {
      "id": "no-weak-crypto-des",
      "path": "\\.go$",
      "pattern": "\"crypto/des\"",
      "message": "DES is not acceptable for new code"
    },
    {
      "id": "no-token-math-rand",
      "path": "\\.go$",
      "pattern": "\"math/rand\"",
      "message": "do not use math/rand for tokens or secrets"
    }
  ],
  "forbiddenPatterns": [],
  "forbiddenPaths": [],
  "forbiddenCommands": [
    {
      "id": "no-destructive-rm",
      "pattern": "rm\\s+-rf\\s+/",
      "message": "verify commands must not remove absolute paths"
    }
  ]
}
`
}

func EnsureGuardrailsScaffold(root string) (bool, error) {
	path := GuardrailsPath(root)
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, AtomicWrite(path, DefaultGuardrailsJSON())
}

func GuardrailsDigest(root string) (string, bool, error) {
	data, err := os.ReadFile(GuardrailsPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", true, err
	}
	return fmt.Sprintf("%x", md5.Sum(data)), true, nil
}

func GateGuardrails(c CheckCtx) (violations, warnings []Violation) {
	return RunGuardrails(c.Root, c.Doc, c.GuardrailsAll)
}

func RunGuardrails(root string, doc *ParsedTasks, all bool) ([]Violation, []Violation) {
	result, violations, warnings := EvaluateGuardrails(root, doc, all)
	_ = result
	return violations, warnings
}

func EvaluateGuardrails(root string, doc *ParsedTasks, all bool) (GuardrailsResult, []Violation, []Violation) {
	cfg, present, err := LoadGuardrailsConfig(root)
	if err != nil {
		return GuardrailsResult{Present: present}, []Violation{guardrailsViolation(".specd/"+guardrailsFileName, err.Error())}, nil
	}
	if !present {
		return GuardrailsResult{}, nil, nil
	}

	compiled, err := compileGuardrails(cfg)
	if err != nil {
		return GuardrailsResult{Present: true}, []Violation{guardrailsViolation(".specd/"+guardrailsFileName, err.Error())}, nil
	}

	files, warnings := guardrailFiles(root, cfg.BaseRef, all)
	violations := evaluateGuardrailFiles(root, files, compiled)
	violations = append(violations, evaluateGuardrailCommands(doc, compiled.commands)...)
	sortGuardrailFindings(violations)
	return GuardrailsResult{Present: true, Warnings: warnings}, violations, warnings
}

func LoadGuardrailsConfig(root string) (GuardrailsConfig, bool, error) {
	path := GuardrailsPath(root)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return GuardrailsConfig{}, false, nil
	}
	if err != nil {
		return GuardrailsConfig{}, true, err
	}
	if len(data) > guardrailsMaxSize {
		return GuardrailsConfig{}, true, fmt.Errorf("file exceeds %d byte limit", guardrailsMaxSize)
	}
	if !utf8.Valid(data) {
		return GuardrailsConfig{}, true, fmt.Errorf("file is not valid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var cfg GuardrailsConfig
	if err := dec.Decode(&cfg); err != nil {
		return GuardrailsConfig{}, true, fmt.Errorf("invalid JSON: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return GuardrailsConfig{}, true, fmt.Errorf("invalid JSON: trailing data")
	} else if !errors.Is(err, io.EOF) {
		return GuardrailsConfig{}, true, fmt.Errorf("invalid JSON: %w", err)
	}
	return cfg, true, nil
}

type compiledGuardrails struct {
	imports  []compiledGuardrailRule
	patterns []compiledGuardrailRule
	paths    []compiledGuardrailRule
	commands []compiledGuardrailRule
}

func compileGuardrails(cfg GuardrailsConfig) (compiledGuardrails, error) {
	imports, err := compileGuardrailRules("forbiddenImports", cfg.ForbiddenImports, true)
	if err != nil {
		return compiledGuardrails{}, err
	}
	patterns, err := compileGuardrailRules("forbiddenPatterns", cfg.ForbiddenPatterns, true)
	if err != nil {
		return compiledGuardrails{}, err
	}
	paths, err := compileGuardrailRules("forbiddenPaths", cfg.ForbiddenPaths, false)
	if err != nil {
		return compiledGuardrails{}, err
	}
	commands, err := compileGuardrailRules("forbiddenCommands", cfg.ForbiddenCommands, false)
	if err != nil {
		return compiledGuardrails{}, err
	}
	return compiledGuardrails{imports: imports, patterns: patterns, paths: paths, commands: commands}, nil
}

func compileGuardrailRules(field string, rules []GuardrailRule, allowPath bool) ([]compiledGuardrailRule, error) {
	out := make([]compiledGuardrailRule, 0, len(rules))
	for i, rule := range rules {
		loc := fmt.Sprintf("%s[%d]", field, i)
		if strings.TrimSpace(rule.Pattern) == "" {
			return nil, fmt.Errorf("%s.pattern is required", loc)
		}
		pat, err := regexp.Compile(rule.Pattern)
		if err != nil {
			return nil, fmt.Errorf("%s.pattern does not compile: %w", loc, err)
		}
		var pathRE *regexp.Regexp
		if strings.TrimSpace(rule.Path) != "" {
			if !allowPath {
				return nil, fmt.Errorf("%s.path is not supported", loc)
			}
			pathRE, err = regexp.Compile(rule.Path)
			if err != nil {
				return nil, fmt.Errorf("%s.path does not compile: %w", loc, err)
			}
		}
		out = append(out, compiledGuardrailRule{rule: rule, pattern: pat, path: pathRE})
	}
	return out, nil
}

func guardrailFiles(root, baseRef string, all bool) ([]string, []Violation) {
	if all {
		return guardrailAllFiles(root), nil
	}
	base := strings.TrimSpace(baseRef)
	if base == "" {
		base = "HEAD"
	}
	cmd := exec.Command("git", "diff", "--name-only", base)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return guardrailAllFiles(root), []Violation{{
			Gate:     "guardrails",
			Location: ".specd/" + guardrailsFileName,
			Message:  "git diff failed; falling back to full scan",
		}}
	}
	files := guardrailCleanFileList(root, strings.Split(string(out), "\n"))
	sort.Strings(files)
	return files, nil
}

func guardrailAllFiles(root string) []string {
	var files []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() && (name == ".git" || name == ".specd") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(files)
	return files
}

func guardrailCleanFileList(root string, raw []string) []string {
	seen := map[string]bool{}
	var files []string
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" || filepath.IsAbs(item) {
			continue
		}
		clean := filepath.ToSlash(filepath.Clean(item))
		if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." || strings.HasPrefix(clean, ".specd/") {
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(clean)))
		if err != nil || info.IsDir() {
			continue
		}
		if !seen[clean] {
			seen[clean] = true
			files = append(files, clean)
		}
	}
	return files
}

func evaluateGuardrailFiles(root string, files []string, rules compiledGuardrails) []Violation {
	var violations []Violation
	for _, rel := range files {
		for _, rule := range rules.paths {
			if rule.pattern.MatchString(rel) {
				violations = append(violations, guardrailRuleViolation(rel, rule.rule))
			}
		}
		if len(rules.imports) == 0 && len(rules.patterns) == 0 {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || !utf8.Valid(data) {
			continue
		}
		text := string(data)
		for _, rule := range rules.imports {
			if rule.path != nil && !rule.path.MatchString(rel) {
				continue
			}
			if rule.pattern.MatchString(text) {
				violations = append(violations, guardrailRuleViolation(rel, rule.rule))
			}
		}
		for _, rule := range rules.patterns {
			if rule.path != nil && !rule.path.MatchString(rel) {
				continue
			}
			if rule.pattern.MatchString(text) {
				violations = append(violations, guardrailRuleViolation(rel, rule.rule))
			}
		}
	}
	return violations
}

func evaluateGuardrailCommands(doc *ParsedTasks, rules []compiledGuardrailRule) []Violation {
	if doc == nil || len(rules) == 0 {
		return nil
	}
	var violations []Violation
	for _, task := range doc.Tasks {
		verify := task.Meta["verify"]
		if verify == "" {
			continue
		}
		for _, rule := range rules {
			if rule.pattern.MatchString(verify) {
				loc := "tasks.md"
				if task.Line > 0 {
					loc = fmt.Sprintf("tasks.md:%d", task.Line)
				}
				violations = append(violations, guardrailRuleViolation(loc, rule.rule))
			}
		}
	}
	return violations
}

func guardrailsViolation(location, message string) Violation {
	return Violation{Gate: "guardrails", Location: location, Message: message}
}

func guardrailRuleViolation(location string, rule GuardrailRule) Violation {
	message := rule.Message
	if strings.TrimSpace(message) == "" {
		message = fmt.Sprintf("forbidden pattern %q", rule.Pattern)
	}
	if strings.TrimSpace(rule.ID) != "" {
		message = rule.ID + ": " + message
	}
	return guardrailsViolation(location, message)
}

func sortGuardrailFindings(violations []Violation) {
	sort.SliceStable(violations, func(i, j int) bool {
		if violations[i].Location != violations[j].Location {
			return violations[i].Location < violations[j].Location
		}
		return violations[i].Message < violations[j].Message
	})
}
