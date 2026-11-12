package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	sqlitedb "github.com/yusuke0627/QueryLab/storage/sqlite"
)

func TestOpenReadOnly(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "original.db")
	writable, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	_, setupErr := writable.Exec("CREATE TABLE users (id INTEGER); INSERT INTO users VALUES (7)")
	if err := errors.Join(setupErr, writable.Close()); err != nil {
		t.Fatal(err)
	}
	// 学習用: URLの区切り文字をファイル名に入れ、接続設定と混同しないか確認します。
	special := filepath.Join(directory, "sample # 100%.db")
	if err := os.Rename(path, special); err != nil {
		t.Fatal(err)
	}
	database, err := sqlitedb.OpenReadOnly(context.Background(), special)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	var id int
	if err := database.QueryRow("SELECT id FROM users").Scan(&id); err != nil || id != 7 {
		t.Fatalf("read id = %d, error = %v", id, err)
	}
	if _, err := database.Exec("INSERT INTO users VALUES (8)"); err == nil {
		t.Fatal("read-only connection accepted a write")
	}
}

func TestOpenReadOnlyRejectsMissingFileAndDSN(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.db")
	for _, path := range []string{"", ":memory:", "file:example.db?mode=rwc", missing} {
		database, err := sqlitedb.OpenReadOnly(context.Background(), path)
		if database != nil {
			database.Close()
		}
		if err == nil || database != nil {
			t.Fatalf("OpenReadOnly(%q) = %v, %v; want nil DB and error", path, database, err)
		}
	}
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing DB should not be created: %v", err)
	}
}

func TestOpenReadOnlyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	database, err := sqlitedb.OpenReadOnly(ctx, filepath.Join(t.TempDir(), "missing.db"))
	if database != nil {
		database.Close()
	}
	if database != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled open = %v, %v", database, err)
	}
}
