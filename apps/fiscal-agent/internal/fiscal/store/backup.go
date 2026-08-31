package store

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BackupFiscalDB is the ONLY fiscal SQLite snapshot path (D6.3): VACUUM INTO.
// destDir empty → {dbDir}/backups.
func (d *DB) BackupFiscalDB(destDir string) (path string, size int64, err error) {
	if d == nil || d.SQL == nil {
		return "", 0, fmt.Errorf("store: db not open")
	}
	if strings.TrimSpace(d.Path) == "" {
		return "", 0, fmt.Errorf("store: db path unknown")
	}
	if destDir == "" {
		destDir = filepath.Join(filepath.Dir(d.Path), "backups")
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", 0, err
	}
	name := fmt.Sprintf("fiscal-%s-%s.db", time.Now().UTC().Format("20060102-150405"), uuidShort())
	path = filepath.Join(destDir, name)
	esc := strings.ReplaceAll(path, `'`, `''`)
	if _, err := d.SQL.Exec(`VACUUM INTO '` + esc + `'`); err != nil {
		return "", 0, fmt.Errorf("store: vacuum into: %w", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		return path, 0, err
	}
	return path, fi.Size(), nil
}

func uuidShort() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}
