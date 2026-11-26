// Package testutil sets up an isolated PostgreSQL transaction for tests that need a real database — every
// repository here talks to Postgres directly (CHECK constraints, unique violations and the foreign keys are part
// of the actual behavior being tested, not something worth mocking away).
//
// Point TEST_DATABASE_URL at a throwaway database (never the dev or
// production one) and NewTx will apply db/migrations/*.sql to it once,
// then hand each test its own transaction that's rolled back on cleanup —
// so tests never see each other's data and can run with t.Parallel().
// Without TEST_DATABASE_URL set (and reachable), tests using this package
// skip instead of failing, so `go test ./...` still works on a machine
// with no Postgres at all.
package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cespedes/cuevasysimas/internal/access"
)

const defaultTestDSN = "postgres://cuevasysimas:cuevasysimas@localhost:64722/cuevasysimas_test?sslmode=disable"

// schemaLockKey is an arbitrary constant used as a Postgres advisory lock
// so that concurrent test binaries (each Go package's tests run in their
// own process) don't race to apply migrations against the same database.
const schemaLockKey = 918_273_645

var (
	setupOnce sync.Once
	setupPool *pgxpool.Pool
	setupErr  error
)

func testDSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return defaultTestDSN
}

// NewTx returns a transaction against a fully migrated test database, with
// automatic rollback registered via t.Cleanup. It calls t.Skip if no test
// database is reachable.
func NewTx(t *testing.T) pgx.Tx {
	t.Helper()
	ctx := context.Background()

	pool, err := ensureSchema(ctx)
	if err != nil {
		t.Skipf("skipping: test database not available at %s (set TEST_DATABASE_URL): %v", testDSN(), err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin test transaction: %v", err)
	}
	t.Cleanup(func() {
		_ = tx.Rollback(context.Background())
	})
	registerIntranet(t, tx)
	return tx
}

// registerIntranet registers the club's intranet as a client of the catalogue inside the transaction of a test
// (the migrations register nobody: a client is registered with cmd/issuer), without a key: the tests that
// check a signature register a key of their own.
func registerIntranet(t *testing.T, tx pgx.Tx) {
	t.Helper()
	ctx := context.Background()
	var account int64
	if err := tx.QueryRow(ctx, `INSERT INTO catalogue.accounts (kind, display_name) VALUES ('service', 'Test client') RETURNING id`).Scan(&account); err != nil {
		t.Fatalf("register the test client: %v", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO catalogue.issuers (issuer, account_id) VALUES ($1, $2)`, access.IntranetIssuer, account); err != nil {
		t.Fatalf("register the test client: %v", err)
	}
}

func ensureSchema(ctx context.Context) (*pgxpool.Pool, error) {
	setupOnce.Do(func() {
		pool, err := pgxpool.New(ctx, testDSN())
		if err != nil {
			setupErr = fmt.Errorf("connect: %w", err)
			return
		}
		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			setupErr = fmt.Errorf("ping: %w", err)
			return
		}

		if err := migrateIfNeeded(ctx, pool); err != nil {
			pool.Close()
			setupErr = err
			return
		}
		setupPool = pool
	})
	return setupPool, setupErr
}

// migrateIfNeeded applies every migration exactly once across however many
// test-binary processes are hitting this database concurrently: it holds a
// session-scoped advisory lock while it checks and, if needed, migrates,
// so a second process always sees the finished result instead of racing
// the first one's DDL.
func migrateIfNeeded(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, int64(schemaLockKey)); err != nil {
		return fmt.Errorf("acquire schema lock: %w", err)
	}
	defer conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, int64(schemaLockKey))

	var migrated bool
	err = conn.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'catalogue' AND table_name = 'place_references')
	`).Scan(&migrated)
	if err != nil {
		return fmt.Errorf("check schema: %w", err)
	}
	if migrated {
		return nil
	}

	files, err := migrationFiles()
	if err != nil {
		return err
	}
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		up, err := upSection(string(content))
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, stmt := range sqlStatements(up) {
			if _, err := conn.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("%s: %w\nstatement:\n%s", path, err, stmt)
			}
		}
	}
	return nil
}

// migrationFiles locates db/migrations relative to this source file, so it
// resolves the same way regardless of which package's tests call NewTx.
func migrationFiles() ([]string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("could not determine testutil source location")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "..", "..", "db", "migrations")

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %s: %w", dir, err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	paths := make([]string, len(names))
	for i, name := range names {
		paths[i] = filepath.Join(dir, name)
	}
	return paths, nil
}

func upSection(content string) (string, error) {
	const upMarker = "-- +goose Up"
	const downMarker = "-- +goose Down"
	start := strings.Index(content, upMarker)
	if start == -1 {
		return "", fmt.Errorf("no %q marker found", upMarker)
	}
	rest := content[start+len(upMarker):]
	if end := strings.Index(rest, downMarker); end != -1 {
		rest = rest[:end]
	}
	return rest, nil
}

const stmtBeginMarker = "-- +goose StatementBegin"
const stmtEndMarker = "-- +goose StatementEnd"

// sqlStatements splits a block of SQL into individual statements. Outside
// a goose StatementBegin/StatementEnd pair, it strips "--" line comments
// first — a semicolon inside one (e.g. "can't be deleted; see below")
// would otherwise look like a statement terminator — then splits on
// plain ";" (none of these migrations put a semicolon inside a string
// literal, so that's otherwise enough). A StatementBegin/StatementEnd
// block (00012_accounting.sql's own trigger function, the one migration
// here that needs it) is passed straight through as a single statement
// instead: goose's own convention for exactly this case, a body that
// contains semicolons of its own ($$-quoted PL/pgSQL) that must reach
// Postgres as one unsplit unit rather than being cut apart the way a
// plain multi-statement block is.
func sqlStatements(sql string) []string {
	var statements []string
	var plain strings.Builder
	flushPlain := func() {
		statements = append(statements, splitPlainStatements(plain.String())...)
		plain.Reset()
	}

	lines := strings.Split(sql, "\n")
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != stmtBeginMarker {
			plain.WriteString(lines[i])
			plain.WriteString("\n")
			continue
		}
		flushPlain()
		var block strings.Builder
		for i++; i < len(lines) && strings.TrimSpace(lines[i]) != stmtEndMarker; i++ {
			block.WriteString(lines[i])
			block.WriteString("\n")
		}
		if stmt := strings.TrimSpace(block.String()); stmt != "" {
			statements = append(statements, stmt)
		}
	}
	flushPlain()
	return statements
}

func splitPlainStatements(sql string) []string {
	var statements []string
	for _, raw := range strings.Split(stripLineComments(sql), ";") {
		stmt := strings.TrimSpace(raw)
		if stmt == "" {
			continue
		}
		statements = append(statements, stmt)
	}
	return statements
}

func stripLineComments(sql string) string {
	lines := strings.Split(sql, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx != -1 {
			lines[i] = line[:idx]
		}
	}
	return strings.Join(lines, "\n")
}
