package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// OpenReadOnly は既存のSQLiteファイルを読み取り専用で開きます。
// 成功時に返すDBを閉じるのは呼び出し元の責務です。
func OpenReadOnly(ctx context.Context, path string) (*sql.DB, error) {
	if ctx == nil {
		return nil, errors.New("open database: context is nil")
	}
	if path == "" || path == ":memory:" || strings.HasPrefix(path, "file:") || strings.ContainsRune(path, 0) {
		return nil, errors.New("open database: expected a local file path")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve database path: %w", err)
	}

	// 学習用: URLを組み立てると、ファイル名中の?や#と接続設定を区別できます。
	uriPath := filepath.ToSlash(absolute)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	dsn := url.URL{Scheme: "file", Path: uriPath, RawQuery: "mode=ro"}
	database, err := sql.Open("sqlite3", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	database.SetMaxOpenConns(1)

	// 学習用: Openだけでは接続が確立していないことがあるので、Pingで確認します。
	if err := database.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("connect to database: %w", errors.Join(err, database.Close()))
	}
	return database, nil
}
