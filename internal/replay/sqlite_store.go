package replay

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(path string, timeout time.Duration) (*SQLiteRepository, error) {
	if path == "" {
		return nil, errors.New("sqlite path is required")
	}
	dsn := sqliteDSN(path, timeout)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	if err := initSQLiteSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &SQLiteRepository{db: db}, nil
}

func (r *SQLiteRepository) Get(ctx context.Context, key string) (StoredResponse, bool, error) {
	var payload []byte
	err := r.db.QueryRowContext(ctx, "SELECT payload FROM flow_items WHERE key = ?", key).Scan(&payload)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return StoredResponse{}, false, nil
		}
		return StoredResponse{}, false, err
	}
	response, err := decodeStoredResponse(payload)
	if err != nil {
		return StoredResponse{}, false, err
	}
	return response, true, nil
}

func (r *SQLiteRepository) Set(ctx context.Context, key string, value StoredResponse, overwrite bool) error {
	payload, err := encodeStoredResponse(value)
	if err != nil {
		return err
	}
	if overwrite {
		_, err = r.db.ExecContext(ctx, `
			INSERT INTO flow_items (key, payload)
			VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET payload = excluded.payload
		`, key, payload)
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO flow_items (key, payload)
		VALUES (?, ?)
	`, key, payload)
	return err
}

func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

func sqliteDSN(path string, timeout time.Duration) string {
	if strings.HasPrefix(path, "file:") || path == ":memory:" {
		return path
	}
	busyMillis := int(timeout.Milliseconds())
	if busyMillis <= 0 {
		busyMillis = 5000
	}
	return fmt.Sprintf("file:%s?_busy_timeout=%d&_foreign_keys=on", path, busyMillis)
}

func initSQLiteSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS flow_items (
			key TEXT PRIMARY KEY,
			payload BLOB NOT NULL
		)
	`)
	return err
}
