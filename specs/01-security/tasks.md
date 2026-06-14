# Stage 01 — Tasks

Branch: `refactor/01-security`. Each task ends with the listed verify command.

## T1 — Centralize env-int parsing (F4)
**Files:** `internal/core/exit.go` or new `internal/core/env.go`; `internal/core/lock.go`; `internal/cmd/verify.go`.

1. Add `internal/core/env.go`:
   ```go
   package core

   import (
       "os"
       "strconv"
   )

   // EnvInt reads name as an int, clamps to [min,max], and warns once on
   // malformed input, returning def. max<=0 means "no upper bound".
   func EnvInt(name string, def, min, max int) int {
       v := os.Getenv(name)
       if v == "" {
           return def
       }
       n, err := strconv.Atoi(v)
       if err != nil {
           Warn(name + ": not an integer (" + v + ") — using default")
           return def
       }
       if n < min {
           n = min
       }
       if max > 0 && n > max {
           n = max
       }
       return n
   }
   ```
2. `lock.go`: replace `numEnv` body to delegate to `EnvInt(name, fallback, 0, 0)`; keep signature or drop `numEnv` and update `staleMs`/`timeoutMs` callers.
3. `verify.go:25-33`: replace `verifyTimeoutMs` internals with
   `core.EnvInt("SPECD_VERIFY_TIMEOUT_MS", 600000, 1, 0)` → `* time.Millisecond`.
4. Add `core.Warn` if it does not exist (check `internal/core/ui.go`); reuse existing logger.

**Verify:** `go test ./internal/core/... && go vet ./...`

## T2 — Honor 0644 in AtomicWrite (F5)
**File:** `internal/core/io.go:31-55`.

1. After `f.Sync()` and before `f.Close()`, add `if err := f.Chmod(0o644); err != nil { return err }`.
2. Add/extend `internal/core/io_test.go`: write a file, `os.Stat`, assert
   `mode.Perm() == 0o644 & ^umask` (read umask via syscall or assert `0o644`
   directly in CI where umask is 022).

**Verify:** `go test ./internal/core/ -run AtomicWrite`

## T3 — Self-update checksum verification (F2)
**File:** `internal/cmd/update.go`.

1. Delete dead block `update.go:48-50` (`if goarch == "amd64"`).
2. Delete the bogus retry `update.go:126-136`; replace with a single
   `downloadBinary` call and a clear error.
3. Add `fetchChecksums(tag string) (map[string]string, error)` that GETs
   `https://github.com/0xkhdr/specd/releases/download/<tag>/SHA256SUMS` and
   parses `"<hex>  <filename>"` lines into a map keyed by filename.
4. In `downloadBinary`: stream the tarball to a temp file under
   `filepath.Dir(destPath)`, compute `sha256.Sum256` while copying (use
   `io.TeeReader` into `sha256.New()`), compare against the entry for
   `tarName`. On mismatch: `os.Remove(tmp)` and
   `return fmt.Errorf("checksum mismatch for %s: got %s want %s", ...)`.
   Only then extract + rename. Fail closed if `SHA256SUMS` is absent.
5. Imports: add `crypto/sha256`, `encoding/hex`.

**Verify:** `go build ./... && go test ./internal/cmd/ -run Update`
(add a test with an `httptest.Server` serving a tarball + matching/ mismatching
SHA256SUMS; assert mismatch leaves no `.new` file and returns error.)

## T4 — Scrub verify child env + pre-run audit (F1)
**File:** `internal/cmd/verify.go:89-101`.

1. Before building `cmd`, reject NUL: `if strings.ContainsRune(command, 0) { return GateError(...) }`.
2. Build a scrubbed env:
   ```go
   func scrubbedEnv() []string {
       allow := []string{"PATH", "HOME", "LANG", "LC_ALL", "TMPDIR"}
       var out []string
       for _, k := range allow {
           if v, ok := os.LookupEnv(k); ok {
               out = append(out, k+"="+v)
           }
       }
       for _, kv := range os.Environ() {
           if strings.HasPrefix(kv, "SPECD_") {
               out = append(out, kv)
           }
       }
       return out
   }
   ```
   Set `cmd.Env = scrubbedEnv()`.
3. Allow shell override: `shell := strings.TrimSpace(os.Getenv("SPECD_VERIFY_SHELL")); if shell == "" { shell = "sh" }` and use `exec.CommandContext(ctx, shell, "-c", command)`.
4. Print before run: `core.Info(fmt.Sprintf("run: %s -c %q  (cwd=%s)", shell, command, root))`.

**Verify:** `go test ./internal/cmd/ -run Verify`

## T5 — Installer checksum verification (F3)
**File:** `scripts/install.sh`.

1. Add `--no-verify` flag parsing (`NO_VERIFY=false`; case `--no-verify) NO_VERIFY=true; shift ;;`).
2. After download (`install.sh:141`), if `NO_VERIFY=false`:
   - download `https://github.com/${REPO}/releases/download/${VERSION}/SHA256SUMS` to `${TMPDIR}/SHA256SUMS`;
   - `cd "$TMPDIR"`; verify with
     `sha256sum --ignore-missing -c SHA256SUMS` (fallback
     `shasum -a 256 -c SHA256SUMS` when `sha256sum` absent);
   - on failure `die "Checksum verification failed for ${ARCHIVE}"`.
3. If `NO_VERIFY=true`, `warn "Skipping checksum verification (--no-verify)"`.
4. Keep POSIX `sh` compliance (no bashisms); test with `sh install.sh --help`-style dry run if available, else `shellcheck scripts/install.sh`.

**Verify:** `shellcheck scripts/install.sh` (if installed) and manual read.

## T6 — Document threat model (F1, F6)
**Files:** `AGENTS.md`, `docs/validation-gates.md`.

1. Add a "Security model" section: `verify:` lines execute as the user via
   `sh -c` with a scrubbed env; only trusted `tasks.md` should be run; slug
   inputs are path-validated; self-update/install verify SHA256.
2. Note lock file content (PID + epoch ms) is non-secret.

**Verify:** `gofmt -l . ` (no Go change) + docs render review.

## Done-when
- `go vet ./... && gofmt -l . && go test -race ./...` all green.
- Update + install both fail closed on checksum mismatch.
- New tests: checksum mismatch (T3), env clamp (T1), file perms (T2), verify env scrub (T4).
