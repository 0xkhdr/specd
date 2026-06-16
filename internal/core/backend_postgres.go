//go:build specd_postgres

// Postgres StateBackend, compiled only under the `specd_postgres` build tag. It
// uses the standard library's database/sql only — no driver is imported here, so
// the project stays dependency-free. To use it, a fork registers a Postgres
// driver via a blank import (e.g. `_ "github.com/jackc/pgx/v5/stdlib"`) in its
// own main package; database/sql then resolves the "pgx"/"postgres" driver name
// at runtime. The default build excludes this file entirely.
package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func init() { registerOptionalBackend("postgres", func() StateBackend { return postgresBackend{} }) }

// postgresBackend stores state as a JSONB-ish text column keyed by slug. The
// lock + CAS spine maps onto Postgres primitives: a transaction-scoped advisory
// lock (pg_advisory_xact_lock) serializes writers, and an UPDATE guarded by the
// monotonic revision is the compare-and-swap (zero rows affected ⇒ conflict).
type postgresBackend struct{}

func (postgresBackend) Name() string { return "postgres" }

func pgOpen() (*sql.DB, error) {
	driver := os.Getenv("SPECD_PG_DRIVER")
	if driver == "" {
		driver = "postgres"
	}
	dsn := os.Getenv("SPECD_PG_DSN")
	if dsn == "" {
		return nil, GateError("postgres backend: SPECD_PG_DSN is unset")
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, GateError(fmt.Sprintf("postgres backend: open: %v", err))
	}
	return db, nil
}

func pgCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func (postgresBackend) Load(_, slug string) (*State, error) {
	db, err := pgOpen()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	ctx, cancel := pgCtx()
	defer cancel()

	var blob string
	err = db.QueryRowContext(ctx, `SELECT doc FROM specd_state WHERE slug = $1`, slug).Scan(&blob)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, GateError(fmt.Sprintf("postgres backend: load %q: %v", slug, err))
	}
	var st State
	if err := json.Unmarshal([]byte(blob), &st); err != nil {
		return nil, GateError(fmt.Sprintf("postgres backend: corrupt state for %q: %v", slug, err))
	}
	return &st, nil
}

func (postgresBackend) Save(_, slug string, state *State) error {
	db, err := pgOpen()
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := pgCtx()
	defer cancel()

	base := state.Revision
	state.Revision++
	state.UpdatedAt = NowISO()
	blob, err := json.Marshal(state)
	if err != nil {
		return err
	}

	// Insert-or-CAS: a first write inserts at the bumped revision; a subsequent
	// write updates only when the stored revision still equals the caller's base.
	res, err := db.ExecContext(ctx, `
		INSERT INTO specd_state (slug, revision, doc) VALUES ($1, $2, $3)
		ON CONFLICT (slug) DO UPDATE SET revision = EXCLUDED.revision, doc = EXCLUDED.doc
		WHERE specd_state.revision = $4`,
		slug, state.Revision, string(blob), base)
	if err != nil {
		return GateError(fmt.Sprintf("postgres backend: save %q: %v", slug, err))
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return GateError(fmt.Sprintf("postgres backend: CAS conflict for %q (stored revision ≠ expected %d) — reload and retry", slug, base))
	}
	return nil
}

func (postgresBackend) WithLock(_, slug string, fn func() error) error {
	db, err := pgOpen()
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), lockTimeoutPG())
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return GateError(fmt.Sprintf("postgres backend: begin: %v", err))
	}
	// Transaction-scoped advisory lock keyed by a stable hash of the slug; it is
	// released automatically when the transaction ends, so no leak on panic.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, slug); err != nil {
		_ = tx.Rollback()
		return GateError(fmt.Sprintf("postgres backend: advisory lock: %v", err))
	}
	if err := fn(); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return GateError(fmt.Sprintf("postgres backend: commit: %v", err))
	}
	return nil
}

func lockTimeoutPG() time.Duration {
	return time.Duration(EnvInt("SPECD_LOCK_TIMEOUT_MS", 5000, 100, 0)) * time.Millisecond
}
