package cmd

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

const githubAPI = "https://api.github.com/repos/0xkhdr/specd/releases/latest"

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func fetchLatestTag() (string, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	req, _ := http.NewRequest("GET", githubAPI, nil)
	req.Header.Set("User-Agent", "specd-cli-updater")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

func downloadBinary(tag, destPath string) error {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	if goarch == "amd64" {
		goarch = "amd64"
	}
	tarName := fmt.Sprintf("specd_%s_%s.tar.gz", goos, goarch)
	url := fmt.Sprintf("https://github.com/0xkhdr/specd/releases/download/%s/%s", tag, tarName)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		if filepath.Base(hdr.Name) == "specd" || filepath.Base(hdr.Name) == "specd.exe" {
			tmp := destPath + ".new"
			f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return fmt.Errorf("create tmp: %w", err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				os.Remove(tmp)
				return fmt.Errorf("write: %w", err)
			}
			f.Close()
			return os.Rename(tmp, destPath)
		}
	}
	return fmt.Errorf("binary not found in archive")
}

func RunUpdate(args cli.Args) int {
	force := args.Bool("force")
	core.Info("Checking for updates...")

	tag, err := fetchLatestTag()
	if err != nil {
		core.Error(fmt.Sprintf("cannot check for updates: %v", err))
		return core.ExitGate
	}

	current := core.Version
	if current == tag && !force {
		core.Success(fmt.Sprintf("Already up to date (%s)", current))
		return core.ExitOK
	}

	core.Info(fmt.Sprintf("Updating %s → %s", current, tag))

	self, err := os.Executable()
	if err != nil {
		core.Error(fmt.Sprintf("cannot locate current binary: %v", err))
		return core.ExitGate
	}
	// Resolve symlinks so we replace the real binary.
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}

	if err := downloadBinary(tag, self); err != nil {
		// Try Windows exe name if the first attempt failed due to wrong extension.
		if !strings.HasSuffix(self, ".exe") {
			if err2 := downloadBinary(tag, self); err2 != nil {
				core.Error(fmt.Sprintf("update failed: %v", err))
				return core.ExitGate
			}
		} else {
			core.Error(fmt.Sprintf("update failed: %v", err))
			return core.ExitGate
		}
	}

	core.Success(fmt.Sprintf("Update complete! specd %s", tag))
	return core.ExitOK
}
