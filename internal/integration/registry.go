package integration

import "github.com/0xkhdr/specd/internal/core"

type Adapter interface {
	Name() string
	Install() InstallRecord
	Snippet(slug, taskID string) string
}

type InstallRecord struct {
	Host  string `json:"host"`
	Owner string `json:"owner"`
}

type Registry struct {
	adapters map[string]Adapter
}

func NewRegistry(adapters ...Adapter) Registry {
	registry := Registry{adapters: map[string]Adapter{}}
	for _, adapter := range adapters {
		registry.adapters[adapter.Name()] = adapter
	}
	return registry
}

func (r Registry) Snippet(host, slug, taskID string) string {
	if adapter, ok := r.adapters[host]; ok {
		return adapter.Snippet(slug, taskID)
	}
	return Snippet(host, slug, taskID)
}

func (r Registry) ModeSnippet(host string, mode core.RequestMode, slug, taskID string, assurance core.AssuranceLevel) string {
	return ModeSnippet(host, mode, slug, taskID, assurance)
}

type StaticAdapter struct {
	Host string
}

func (a StaticAdapter) Name() string {
	return a.Host
}

func (a StaticAdapter) Install() InstallRecord {
	return InstallRecord{Host: a.Host, Owner: "specd"}
}

func (a StaticAdapter) Snippet(slug, taskID string) string {
	return Snippet(a.Host, slug, taskID)
}
