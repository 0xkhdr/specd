package version

import (
	"runtime"
	"runtime/debug"
)

var (
	// Version is injected by release builds with:
	// -X github.com/0xkhdr/specd/internal/version.Version=<tag>
	Version = "dev"
	Commit  = ""
	Date    = ""
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitempty"`
	Date    string `json:"date,omitempty"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	Dirty   bool   `json:"dirty,omitempty"`
}

func Get() Info {
	info := Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
		Go:      runtime.Version(),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit == "" {
					info.Commit = setting.Value
				}
			case "vcs.modified":
				info.Dirty = setting.Value == "true"
			}
		}
	}
	return info
}
