package cmd

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
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

// releaseBaseURL is the GitHub release-download prefix; overridable in tests.
var releaseBaseURL = "https://github.com/0xkhdr/specd/releases/download"

// releaseURL builds the download URL for an asset of the given release tag.
func releaseURL(tag, asset string) string {
	return fmt.Sprintf("%s/%s/%s", releaseBaseURL, tag, asset)
}

// fetchChecksums downloads the SHA256SUMS file for tag and parses its
// "<hex>  <filename>" lines into a map keyed by filename.
func fetchChecksums(tag string) (map[string]string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(releaseURL(tag, "SHA256SUMS"))
	if err != nil {
		return nil, fmt.Errorf("fetch SHA256SUMS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch SHA256SUMS: HTTP %d (release must publish checksums)", resp.StatusCode)
	}
	sums := map[string]string{}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		// "<hex>  <filename>"; filename may be prefixed with "*" for binary mode.
		sums[strings.TrimPrefix(fields[1], "*")] = strings.ToLower(fields[0])
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("parse SHA256SUMS: %w", err)
	}
	if len(sums) == 0 {
		return nil, fmt.Errorf("SHA256SUMS is empty")
	}
	return sums, nil
}

func downloadBinary(tag, destPath string) error {
	tarName := fmt.Sprintf("specd_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)

	// Fail closed: no verified checksum, no install.
	sums, err := fetchChecksums(tag)
	if err != nil {
		return err
	}
	want, ok := sums[tarName]
	if !ok {
		return fmt.Errorf("no checksum for %s in SHA256SUMS", tarName)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(releaseURL(tag, tarName))
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download %s: HTTP %d", tarName, resp.StatusCode)
	}

	// Stream the tarball to a temp file beside destPath, hashing as we go.
	tmpTar, err := os.CreateTemp(filepath.Dir(destPath), ".specd-dl-*.tar.gz")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpTarName := tmpTar.Name()
	defer os.Remove(tmpTarName)

	h := sha256.New()
	if _, err := io.Copy(tmpTar, io.TeeReader(resp.Body, h)); err != nil {
		_ = tmpTar.Close()
		return fmt.Errorf("download: %w", err)
	}
	if err := tmpTar.Close(); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	got := hex.EncodeToString(h.Sum(nil))
	if got != want {
		return fmt.Errorf("checksum mismatch for %s: got %s want %s", tarName, got, want)
	}

	// Verified — extract the binary and atomically replace destPath.
	return extractBinary(tmpTarName, destPath)
}

// maxBinarySize caps how many bytes are extracted from the release tarball, so a
// crafted archive cannot exhaust disk via a decompression bomb. 512 MiB is well
// above any real specd binary.
const maxBinarySize = 512 << 20

// extractBinary reads the verified tarball at tarPath, finds the specd binary,
// writes it to destPath+".new", and renames it over destPath.
//
// KNOWN LIMITATION (Windows): the final os.Rename replaces the running
// executable in place. POSIX permits renaming over a running binary; Windows
// locks an in-use .exe and the rename fails with "Access is denied". specd
// therefore builds and runs on Windows, but `specd update` self-replacement is
// known-limited there — Windows users should reinstall from a fresh download
// instead. Lifting this needs the rename-to-sidecar + relaunch dance; tracked
// as follow-up (see TESTING.md → "Windows limitation"), not silently broken.
func extractBinary(tarPath, destPath string) error {
	tf, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = tf.Close() }()

	gz, err := gzip.NewReader(tf)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

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
			// 0o755: this is an executable we will rename over the running binary.
			f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //nolint:gosec // G302: the extracted artifact is an executable binary and must be executable.
			if err != nil {
				return fmt.Errorf("create tmp: %w", err)
			}
			// Cap extraction to guard against a decompression bomb (G110).
			if _, err := io.Copy(f, io.LimitReader(tr, maxBinarySize)); err != nil {
				_ = f.Close()
				os.Remove(tmp)
				return fmt.Errorf("write: %w", err)
			}
			_ = f.Close()
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
		core.Error(fmt.Sprintf("update failed: %v", err))
		return core.ExitGate
	}

	core.Success(fmt.Sprintf("Update complete! specd %s", tag))
	return core.ExitOK
}
