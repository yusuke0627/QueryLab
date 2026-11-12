package execution_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-sqlite3"
	q "github.com/yusuke0627/QueryLab"
	"github.com/yusuke0627/QueryLab/execution"
	sqlitedb "github.com/yusuke0627/QueryLab/storage/sqlite"
)

func fixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.db")
	database, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	_, setupErr := database.Exec(`
CREATE TABLE users (id INTEGER, name TEXT, active INTEGER, deleted_at TEXT, payload BLOB);
INSERT INTO users VALUES (1, 'Alice', 1, NULL, X'0102'), (2, 'O''Brien ?', 0, NULL, X'0304'),
                         (3, 'Carol', 1, '2026-11-01', NULL);
CREATE TABLE many (id INTEGER);
WITH RECURSIVE numbers(id) AS (VALUES(1) UNION ALL SELECT id+1 FROM numbers WHERE id<1002)
INSERT INTO many SELECT id FROM numbers;
CREATE VIEW cancel_during_read AS SELECT 1 AS id UNION ALL SELECT cancel_read();
`)
	if err := errors.Join(setupErr, database.Close()); err != nil {
		t.Fatal(err)
	}
	return path
}

func openFixture(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sqlitedb.OpenReadOnly(context.Background(), fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func ids(table string) q.Select {
	return q.Select{
		From:    q.Table{Name: table},
		Columns: []q.Projection{{Expr: q.Col("id")}},
		OrderBy: []q.Order{{Expr: q.Col("id")}},
	}
}

func TestExecutePreservesColumnPositionsAndValues(t *testing.T) {
	database := openFixture(t)
	query := ids("users")
	query.Columns = []q.Projection{
		{Expr: q.Col("id"), Alias: "same"},
		{Expr: q.Col("name"), Alias: "same"},
		{Expr: q.Col("deleted_at")},
		{Expr: q.Col("payload")},
	}
	result, err := execution.Execute(context.Background(), database, query, execution.Options{})
	if err != nil {
		t.Fatal(err)
	}
	wantColumns := []execution.Column{
		{Name: "same", DatabaseTypeName: "INTEGER"},
		{Name: "same", DatabaseTypeName: "TEXT"},
		{Name: "deleted_at", DatabaseTypeName: "TEXT"},
		{Name: "payload", DatabaseTypeName: "BLOB"},
	}
	wantRows := [][]any{
		{int64(1), "Alice", nil, []byte{1, 2}},
		{int64(2), "O'Brien ?", nil, []byte{3, 4}},
		{int64(3), "Carol", "2026-11-01", nil},
	}
	if result.Truncated || !reflect.DeepEqual(result.Columns, wantColumns) || !reflect.DeepEqual(result.Rows, wantRows) {
		t.Fatalf("unexpected result: %#v", result)
	}

	// 学習用: 片方の結果を変更しても、別の行や次回の取得結果には影響しません。
	result.Rows[0][3].([]byte)[0] = 99
	if !reflect.DeepEqual(result.Rows[1], wantRows[1]) {
		t.Fatal("two rows share their byte buffers")
	}
	query.Where = q.Ptr(q.Eq(q.Col("name"), q.Text("O'Brien ?")))
	result, err = execution.Execute(context.Background(), database, query, execution.Options{})
	if err != nil || !reflect.DeepEqual(result.Rows, wantRows[1:2]) {
		t.Fatalf("bound text: %#v, %v", result, err)
	}
	query.Where = q.Ptr(q.And(q.Eq(q.Col("active"), q.Bool(true)), q.IsNull(q.Col("deleted_at"))))
	result, err = execution.Execute(context.Background(), database, query, execution.Options{})
	if err != nil || !reflect.DeepEqual(result.Rows, wantRows[:1]) {
		t.Fatalf("bound boolean and NULL filter: %#v, %v", result, err)
	}
}

func TestExecuteLimits(t *testing.T) {
	database := openFixture(t)
	tests := []struct {
		name      string
		table     string
		maxRows   int
		limit     *int64
		want      int
		truncated bool
	}{
		{name: "below cap", table: "users", maxRows: 4, want: 3},
		{name: "exact cap", table: "users", maxRows: 3, want: 3},
		{name: "above cap", table: "users", maxRows: 2, want: 2, truncated: true},
		{name: "default cap", table: "many", want: 1000, truncated: true},
		{name: "model limit", table: "users", maxRows: 2, limit: q.Ptr(int64(2)), want: 2},
		{name: "model limit zero", table: "users", limit: q.Ptr(int64(0))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query := ids(test.table)
			query.Limit = test.limit
			result, err := execution.Execute(context.Background(), database, query, execution.Options{MaxRows: test.maxRows})
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Columns) != 1 || result.Rows == nil || len(result.Rows) != test.want || result.Truncated != test.truncated {
				t.Fatalf("got %d columns, %d rows, truncated=%t", len(result.Columns), len(result.Rows), result.Truncated)
			}
			if test.want > 0 && result.Rows[test.want-1][0] != int64(test.want) {
				t.Fatalf("unexpected final row: %#v", result.Rows[test.want-1])
			}
			if query.Limit != test.limit {
				t.Fatal("query limit was changed")
			}
		})
	}
}

func TestExecuteErrorsReturnNoPartialResult(t *testing.T) {
	database := openFixture(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name    string
		ctx     context.Context
		query   q.Select
		options execution.Options
		message string
		cause   error
	}{
		{name: "missing table", query: ids("missing"), message: "no such table"},
		{name: "invalid model", message: "compile select"},
		{name: "negative limit", query: ids("users"), options: execution.Options{MaxRows: -1}, message: "must not be negative"},
		{name: "negative timeout", query: ids("users"), options: execution.Options{Timeout: -1}, message: "must not be negative"},
		{name: "pre-canceled", ctx: canceled, query: ids("users"), cause: context.Canceled},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := test.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			result, err := execution.Execute(ctx, database, test.query, test.options)
			if err == nil || !strings.Contains(err.Error(), test.message) ||
				(test.cause != nil && !errors.Is(err, test.cause)) || !reflect.DeepEqual(result, execution.Result{}) {
				t.Fatalf("Execute = %#v, %v", result, err)
			}
		})
	}
	if _, err := execution.Execute(context.Background(), database, ids("users"), execution.Options{}); err != nil {
		t.Fatalf("reuse after failures: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
	if result, err := execution.Execute(context.Background(), database, ids("users"), execution.Options{}); err == nil || !reflect.DeepEqual(result, execution.Result{}) {
		t.Fatalf("closed DB: %#v, %v", result, err)
	}
}

func TestExecuteTimeoutWaitingForConnection(t *testing.T) {
	database := openFixture(t)
	connection, err := database.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()

	// 学習用: 1接続を借りたままにするので、sleepを使わず待機中の期限切れを確認できます。
	result, err := execution.Execute(context.Background(), database, ids("users"), execution.Options{Timeout: time.Millisecond})
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(result, execution.Result{}) {
		t.Fatalf("timeout = %#v, %v", result, err)
	}
}

func TestExecuteCancelDuringRead(t *testing.T) {
	path := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// 学習用: SQLiteが2行目を評価したときに中止します。時刻の推測やsleepは使いません。
	database := sql.OpenDB(testConnector{
		source: path,
		implementation: &sqlite3.SQLiteDriver{ConnectHook: func(connection *sqlite3.SQLiteConn) error {
			return connection.RegisterFunc("cancel_read", func() int64 {
				cancel()
				return 2
			}, false)
		}},
	})
	defer database.Close()
	database.SetMaxOpenConns(1)
	query := ids("cancel_during_read")
	query.OrderBy = nil
	result, err := execution.Execute(ctx, database, query, execution.Options{})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, execution.Result{}) {
		t.Fatalf("cancel during read: %#v, %v", result, err)
	}
	if _, err := execution.Execute(context.Background(), database, ids("users"), execution.Options{}); err != nil {
		t.Fatalf("rows not released after cancellation: %v", err)
	}
}

type testConnector struct {
	source         string
	implementation driver.Driver
}

func (connector testConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return connector.implementation.Open(connector.source)
}

func (connector testConnector) Driver() driver.Driver {
	return connector.implementation
}
