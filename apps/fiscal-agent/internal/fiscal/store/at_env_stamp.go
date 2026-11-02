package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const metaKeyATEnv = "at_env"

// EnsureATEnvStamp is the ONLY binder of one fiscal.db to one AT env (mock|test|prod).
// First open stamps the DB; later opens with a different env fail closed so mock
// validation codes / test hash chains never mix with prod invoices.
// Unstamped legacy DBs that already hold series or invoices are treated as mock
// (the only env that could issue before A1).
func (d *DB) EnsureATEnvStamp(env string) error {
	env = strings.ToLower(strings.TrimSpace(env))
	if env == "" {
		return fmt.Errorf("store: at env required")
	}
	var bound string
	err := d.SQL.QueryRow(`SELECT value FROM fiscal_meta WHERE key = ?`, metaKeyATEnv).Scan(&bound)
	switch {
	case err == nil:
	case errors.Is(err, sql.ErrNoRows):
		bound = env
		var n int
		if err := d.SQL.QueryRow(`SELECT (SELECT COUNT(1) FROM series) + (SELECT COUNT(1) FROM invoices)`).Scan(&n); err != nil {
			return fmt.Errorf("store: at env stamp: %w", err)
		}
		if n > 0 {
			bound = "mock"
		}
		if _, err := d.SQL.Exec(
			`INSERT INTO fiscal_meta (key, value, updated_at) VALUES (?, ?, ?)`,
			metaKeyATEnv, bound, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("store: at env stamp: %w", err)
		}
	default:
		return fmt.Errorf("store: at env stamp: %w", err)
	}
	if bound != env {
		return fmt.Errorf("store: %s is bound to AT env %q but fiscal_at_env=%q; restore config.json fiscal_at_env or use the %s database", d.Path, bound, env, env)
	}
	return nil
}
