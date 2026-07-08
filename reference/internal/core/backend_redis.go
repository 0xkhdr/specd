//go:build specd_redis

// Package-local Redis StateBackend, compiled only under the `specd_redis` build
// tag. It speaks the RESP wire protocol over a raw net.Conn using nothing but
// the standard library, so even the tagged build pulls in no third-party redis
// driver — the project stays dependency-free. The default build excludes this
// file entirely.
package core

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

func init() { registerOptionalBackend("redis", func() StateBackend { return redisBackend{} }) }

// redisBackend stores each spec's state as a JSON string under a namespaced key
// and implements the lock + CAS spine with a SET NX lock and a WATCH/MULTI/EXEC
// compare-and-swap on the state's monotonic revision.
type redisBackend struct{}

func (redisBackend) Name() string { return "redis" }

func redisKey(slug string) string {
	prefix := os.Getenv("SPECD_REDIS_PREFIX")
	if prefix == "" {
		prefix = "specd"
	}
	return prefix + ":state:" + slug
}

func (redisBackend) Load(_, slug string) (*State, error) {
	c, err := dialRedis()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	raw, ok, err := c.getBulk(redisKey(slug))
	if err != nil || !ok {
		return nil, err
	}
	var st State
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return nil, GateError(fmt.Sprintf("redis backend: corrupt state for %q: %v", slug, err))
	}
	return &st, nil
}

func (redisBackend) Save(_, slug string, state *State) error {
	c, err := dialRedis()
	if err != nil {
		return err
	}
	defer c.Close()
	key := redisKey(slug)

	// Optimistic CAS on revision: WATCH the key, verify the stored revision
	// matches the caller's base, bump it, then commit in a MULTI/EXEC. A nil EXEC
	// (the key changed under us) is a conflict, mirroring the file backend.
	if err := c.cmd("WATCH", key); err != nil {
		return err
	}
	cur, ok, err := c.getBulk(key)
	if err != nil {
		return err
	}
	if ok {
		var onDisk State
		if err := json.Unmarshal([]byte(cur), &onDisk); err != nil {
			return GateError(fmt.Sprintf("redis backend: corrupt state for %q: %v", slug, err))
		}
		if onDisk.Revision != state.Revision {
			_ = c.cmd("UNWATCH")
			return GateError(fmt.Sprintf("redis backend: state for %q changed underfoot (stored revision %d ≠ expected %d) — reload and retry", slug, onDisk.Revision, state.Revision))
		}
	}
	state.Revision++
	state.UpdatedAt = NowISO()
	blob, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := c.cmd("MULTI"); err != nil {
		return err
	}
	if err := c.cmd("SET", key, string(blob)); err != nil {
		return err
	}
	committed, err := c.execMulti()
	if err != nil {
		return err
	}
	if !committed {
		return GateError(fmt.Sprintf("redis backend: CAS conflict committing state for %q — reload and retry", slug))
	}
	return nil
}

// reentrant lock bookkeeping, mirroring the file backend's owning-goroutine
// reentrancy so nested WithLock calls do not deadlock.
var (
	redisLockMu sync.Mutex
	redisHeld   = map[string]int{} // slug -> reentrancy depth for the holding goroutine
)

func (redisBackend) WithLock(_, slug string, fn func() error) error {
	redisLockMu.Lock()
	if redisHeld[slug] > 0 {
		redisHeld[slug]++
		redisLockMu.Unlock()
		defer func() {
			redisLockMu.Lock()
			redisHeld[slug]--
			redisLockMu.Unlock()
		}()
		return fn()
	}
	redisLockMu.Unlock()

	c, err := dialRedis()
	if err != nil {
		return err
	}
	defer c.Close()

	lockKey := redisKey(slug) + ":lock"
	token := randToken()
	deadline := time.Now().Add(lockTimeout())
	for {
		ok, err := c.setNXPX(lockKey, token, lockStaleMs())
		if err != nil {
			return err
		}
		if ok {
			break
		}
		if time.Now().After(deadline) {
			return GateError(fmt.Sprintf("redis backend: lock for %q not acquired within timeout", slug))
		}
		time.Sleep(10 * time.Millisecond)
	}
	redisLockMu.Lock()
	redisHeld[slug] = 1
	redisLockMu.Unlock()
	defer func() {
		redisLockMu.Lock()
		redisHeld[slug] = 0
		redisLockMu.Unlock()
		_ = c.cmd("DEL", lockKey) // best-effort release; the PX TTL is the backstop
	}()
	return fn()
}

func lockTimeout() time.Duration {
	return time.Duration(EnvInt("SPECD_LOCK_TIMEOUT_MS", 5000, 100, 0)) * time.Millisecond
}
func lockStaleMs() int { return EnvInt("SPECD_LOCK_STALE_MS", 10000, 100, 0) }

func randToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// --- minimal RESP client (stdlib only) ---

type redisConn struct {
	conn net.Conn
	r    *bufio.Reader
}

func dialRedis() (*redisConn, error) {
	addr := os.Getenv("SPECD_REDIS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, GateError(fmt.Sprintf("redis backend: dial %s: %v", addr, err))
	}
	return &redisConn{conn: conn, r: bufio.NewReader(conn)}, nil
}

func (c *redisConn) Close() error { return c.conn.Close() }

func (c *redisConn) write(args ...string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	_, err := c.conn.Write([]byte(b.String()))
	return err
}

// cmd sends a command and discards a single non-error reply.
func (c *redisConn) cmd(args ...string) error {
	if err := c.write(args...); err != nil {
		return err
	}
	_, _, err := c.readReply()
	return err
}

func (c *redisConn) getBulk(key string) (string, bool, error) {
	if err := c.write("GET", key); err != nil {
		return "", false, err
	}
	s, null, err := c.readReply()
	if err != nil {
		return "", false, err
	}
	return s, !null, nil
}

func (c *redisConn) setNXPX(key, val string, ms int) (bool, error) {
	if err := c.write("SET", key, val, "NX", "PX", strconv.Itoa(ms)); err != nil {
		return false, err
	}
	s, null, err := c.readReply()
	if err != nil {
		return false, err
	}
	return !null && s == "OK", nil
}

// execMulti sends EXEC and reports whether the transaction committed (a nil
// array reply means a WATCHed key changed and the transaction was aborted).
func (c *redisConn) execMulti() (bool, error) {
	if err := c.write("EXEC"); err != nil {
		return false, err
	}
	_, null, err := c.readReply()
	if err != nil {
		return false, err
	}
	return !null, nil
}

// readReply parses one RESP reply. It returns the string payload (for simple
// strings, bulk strings, and integers) and a null flag (for $-1 / *-1).
func (c *redisConn) readReply() (string, bool, error) {
	line, err := c.r.ReadString('\n')
	if err != nil {
		return "", false, err
	}
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return "", false, GateError("redis backend: empty reply")
	}
	switch line[0] {
	case '+', ':':
		return line[1:], false, nil
	case '-':
		return "", false, GateError("redis backend: " + line[1:])
	case '$':
		n, _ := strconv.Atoi(line[1:])
		if n < 0 {
			return "", true, nil
		}
		buf := make([]byte, n+2) // include trailing CRLF
		if _, err := readFull(c.r, buf); err != nil {
			return "", false, err
		}
		return string(buf[:n]), false, nil
	case '*':
		n, _ := strconv.Atoi(line[1:])
		if n < 0 {
			return "", true, nil
		}
		// Drain array elements; callers only need the null/non-null distinction.
		for i := 0; i < n; i++ {
			if _, _, err := c.readReply(); err != nil {
				return "", false, err
			}
		}
		return "", false, nil
	default:
		return "", false, GateError("redis backend: unexpected reply: " + line)
	}
}

func readFull(r *bufio.Reader, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
