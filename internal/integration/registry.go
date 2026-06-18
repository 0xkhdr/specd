package integration

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Registry stores adapters by stable CLI name.
type Registry struct {
	adapters map[string]HostAdapter
}

func NewRegistry(adapters ...HostAdapter) (*Registry, error) {
	registry := &Registry{adapters: make(map[string]HostAdapter, len(adapters))}
	for _, adapter := range adapters {
		if adapter == nil {
			return nil, fmt.Errorf("host adapter is nil")
		}
		name := adapter.Name()
		if name == "" {
			return nil, fmt.Errorf("host adapter name is empty")
		}
		if name != strings.ToLower(name) || strings.TrimSpace(name) != name {
			return nil, fmt.Errorf("host adapter name %q must be lowercase and trimmed", name)
		}
		if _, exists := registry.adapters[name]; exists {
			return nil, fmt.Errorf("duplicate host adapter %q", name)
		}
		scopes := adapter.Scopes()
		if len(scopes) == 0 {
			return nil, fmt.Errorf("host adapter %q has no supported scopes", name)
		}
		seen := map[Scope]bool{}
		for _, scope := range scopes {
			if scope != ScopeProject && scope != ScopeGlobal {
				return nil, fmt.Errorf("host adapter %q has invalid scope %q", name, scope)
			}
			if seen[scope] {
				return nil, fmt.Errorf("host adapter %q repeats scope %q", name, scope)
			}
			seen[scope] = true
		}
		registry.adapters[name] = adapter
	}
	return registry, nil
}

func MustRegistry(adapters ...HostAdapter) *Registry {
	registry, err := NewRegistry(adapters...)
	if err != nil {
		panic(err)
	}
	return registry
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Registry) Get(name string) (HostAdapter, bool) {
	adapter, ok := r.adapters[name]
	return adapter, ok
}

func (r *Registry) Adapters() []HostAdapter {
	names := r.Names()
	adapters := make([]HostAdapter, 0, len(names))
	for _, name := range names {
		adapters = append(adapters, r.adapters[name])
	}
	return adapters
}

func (r *Registry) Detect(root string) []Detection {
	return DetectAll(r, root)
}

func (r *Registry) Plan(name, root string, scope Scope) (HostPlan, error) {
	adapter, ok := r.Get(name)
	if !ok {
		return HostPlan{}, fmt.Errorf("unsupported host %q", name)
	}
	supported := false
	for _, candidate := range adapter.Scopes() {
		if candidate == scope {
			supported = true
			break
		}
	}
	if !supported {
		return HostPlan{}, fmt.Errorf("host %q does not support %s scope", name, scope)
	}
	plan, err := adapter.Plan(root, scope)
	if err != nil {
		return HostPlan{}, err
	}
	if err := validatePlan(adapter, root, scope, plan); err != nil {
		return HostPlan{}, err
	}
	return normalizePlan(plan), nil
}

func (r *Registry) Install(ctx context.Context, plan HostPlan) (HostResult, error) {
	adapter, ok := r.Get(plan.Host)
	if !ok {
		return HostResult{}, fmt.Errorf("unsupported host %q", plan.Host)
	}
	if err := validatePlan(adapter, plan.Root, plan.Scope, plan); err != nil {
		return HostResult{}, err
	}
	result, err := adapter.Install(ctx, normalizePlan(plan))
	if err != nil {
		return HostResult{}, err
	}
	if result.Targets == nil {
		result.Targets = []string{}
	}
	if result.Backups == nil {
		result.Backups = []string{}
	}
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	sort.Strings(result.Targets)
	sort.Strings(result.Backups)
	sort.Strings(result.Warnings)
	return result, nil
}
