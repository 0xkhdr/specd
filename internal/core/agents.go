package core

import (
	"os"
	"path/filepath"
	"strings"
)

type AgentHost struct {
	Name    string
	Detect  string
	Plan    string
	Install string
	Verify  string
}

func AgentHosts() []AgentHost {
	return []AgentHost{
		{Name: "codex", Detect: "codex", Plan: "read tasks.md frontier", Install: "none", Verify: "specd verify"},
		{Name: "claude", Detect: "claude", Plan: "read tasks.md frontier", Install: "none", Verify: "specd verify"},
		{Name: "pinky", Detect: "pinky", Plan: "role-scoped worker artifacts", Install: "claude+codex", Verify: "specd verify"},
	}
}

const (
	agentsBegin = "<!-- specd:agents begin -->"
	agentsEnd   = "<!-- specd:agents end -->"
)

func MergeAgents(existing, generated string) string {
	block := agentsBegin + "\n" + strings.TrimSpace(generated) + "\n" + agentsEnd
	start := strings.Index(existing, agentsBegin)
	if start < 0 {
		if strings.TrimSpace(existing) == "" {
			return block + "\n"
		}
		return strings.TrimRight(existing, "\n") + "\n\n" + block + "\n"
	}
	end := strings.Index(existing[start:], agentsEnd)
	if end < 0 {
		return strings.TrimRight(existing, "\n") + "\n\n" + block + "\n"
	}
	end += start + len(agentsEnd)
	return existing[:start] + block + existing[end:]
}

const (
	pinkyCodexBegin = "# specd:pinky begin"
	pinkyCodexEnd   = "# specd:pinky end"
)

func MergePinkyCodexConfig(existing, root string) string {
	// Only the MCP server registration lives here. Codex auto-discovers the
	// per-role agent definitions from .codex/agents/*.toml, so declaring them
	// again in config.toml made codex see each role twice ("duplicate agent
	// role name ... in the same config layer") and drop them.
	block := strings.Join([]string{
		pinkyCodexBegin,
		`[mcp_servers.specd]`,
		`command = "specd"`,
		`args = ["mcp"]`,
		`cwd = "` + root + `"`,
		pinkyCodexEnd,
	}, "\n")
	start := strings.Index(existing, pinkyCodexBegin)
	if start < 0 {
		if strings.TrimSpace(existing) == "" {
			return block + "\n"
		}
		return strings.TrimRight(existing, "\n") + "\n\n" + block + "\n"
	}
	end := strings.Index(existing[start:], pinkyCodexEnd)
	if end < 0 {
		return strings.TrimRight(existing, "\n") + "\n\n" + block + "\n"
	}
	end += start + len(pinkyCodexEnd)
	return existing[:start] + block + existing[end:]
}

type AgentDiscovery struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"`
	Files   []string `json:"files,omitempty"`
	Missing []string `json:"missing,omitempty"`
	Invalid []string `json:"invalid,omitempty"`
}

// WorkerDefinitions is the filesystem-backed worker-presence probe shared by
// doctor and orchestration. It deliberately exposes only WorkerAvailable to
// the controller: harness detection and path conventions stay in core, while
// orchestration depends on a tiny interface and never imports cmd.
type WorkerDefinitions struct {
	Root    string
	Harness string
}

// WorkerAvailable reports whether every Pinky role required by the active
// harness is installed. Unknown harnesses fail closed.
func (w WorkerDefinitions) WorkerAvailable() bool {
	missing, invalid := w.Problems()
	return len(missing) == 0 && len(invalid) == 0
}

// Problems returns stable workspace-relative paths. Codex additionally needs
// its registration block; Claude discovers workers directly from agent files.
func (w WorkerDefinitions) Problems() (missing, invalid []string) {
	paths, known := pinkyFilesForHarness(w.Harness)
	if !known {
		return nil, []string{"handshake agent " + w.Harness + " is not a supported worker harness (want codex or claude)"}
	}
	for _, rel := range paths {
		if _, err := os.Stat(filepath.Join(w.Root, rel)); err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, rel)
			} else {
				invalid = append(invalid, rel+": "+err.Error())
			}
		}
	}
	if strings.EqualFold(w.Harness, "codex") {
		const rel = ".codex/config.toml"
		raw, err := os.ReadFile(filepath.Join(w.Root, rel))
		switch {
		case os.IsNotExist(err):
			missing = append(missing, rel)
		case err != nil:
			invalid = append(invalid, rel+": "+err.Error())
		case !validPinkyCodexConfig(string(raw)):
			invalid = append(invalid, rel+": missing pinky managed block")
		}
	}
	return missing, invalid
}

func pinkyFilesForHarness(harness string) ([]string, bool) {
	var ext, dir string
	switch strings.ToLower(strings.TrimSpace(harness)) {
	case "claude":
		dir, ext = ".claude/agents", ".md"
	case "codex":
		dir, ext = ".codex/agents", ".toml"
	default:
		return nil, false
	}
	paths := make([]string, 0, 4)
	for _, role := range []string{"scout", "craftsman", "validator", "auditor"} {
		paths = append(paths, dir+"/pinky-"+role+ext)
	}
	return paths, true
}

func DiscoverAgents(root string) []AgentDiscovery {
	pinky := AgentDiscovery{Name: "pinky", Files: pinkyFiles()}
	for _, rel := range pinky.Files {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			if os.IsNotExist(err) {
				pinky.Missing = append(pinky.Missing, rel)
				continue
			}
			pinky.Invalid = append(pinky.Invalid, rel+": "+err.Error())
		}
	}
	configPath := filepath.Join(root, ".codex", "config.toml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			pinky.Missing = append(pinky.Missing, ".codex/config.toml")
		} else {
			pinky.Invalid = append(pinky.Invalid, ".codex/config.toml: "+err.Error())
		}
	} else if !validPinkyCodexConfig(string(config)) {
		pinky.Invalid = append(pinky.Invalid, ".codex/config.toml: missing pinky managed block")
	}
	switch {
	case len(pinky.Invalid) > 0:
		pinky.Status = "invalid"
	case len(pinky.Missing) > 0:
		pinky.Status = "missing"
	default:
		pinky.Status = "installed"
	}
	return []AgentDiscovery{pinky}
}

func pinkyFiles() []string {
	return []string{
		".claude/agents/pinky-scout.md",
		".claude/agents/pinky-craftsman.md",
		".claude/agents/pinky-validator.md",
		".claude/agents/pinky-auditor.md",
		".codex/agents/pinky-scout.toml",
		".codex/agents/pinky-craftsman.toml",
		".codex/agents/pinky-validator.toml",
		".codex/agents/pinky-auditor.toml",
	}
}

func validPinkyCodexConfig(config string) bool {
	if !strings.Contains(config, pinkyCodexBegin) || !strings.Contains(config, pinkyCodexEnd) {
		return false
	}
	return strings.Contains(config, "[mcp_servers.specd]")
}
