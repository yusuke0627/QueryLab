package execution_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yusuke0627/QueryLab/execution"
)

func TestExecuteClosesRowsOnCollectionFailure(t *testing.T) {
	failure := errors.New("test driver failure")
	for _, phase := range []string{"scan row", "read rows", "close rows"} {
		t.Run(phase, func(t *testing.T) {
			rows := &faultRows{phase: phase, failure: failure}
			database := sql.OpenDB(testConnector{implementation: faultDriver{rows: rows}})
			defer database.Close()

			result, err := execution.Execute(context.Background(), database, ids("users"), execution.Options{MaxRows: 1})
			if err == nil || !strings.Contains(err.Error(), phase) || !reflect.DeepEqual(result, execution.Result{}) {
				t.Fatalf("Execute = %#v, %v; want failure at %s", result, err, phase)
			}
			if phase != "scan row" && !errors.Is(err, failure) {
				t.Fatalf("original cause was lost: %v", err)
			}
			if !rows.closed.Load() {
				t.Fatal("rows not closed")
			}
		})
	}
}

// 学習用: 実SQLiteでは狙った位置のエラーを起こしにくいので、このテストだけ偽物を使います。
type faultDriver struct {
	rows *faultRows
}

func (implementation faultDriver) Open(string) (driver.Conn, error) {
	return faultConn{rows: implementation.rows}, nil
}

type faultConn struct {
	rows *faultRows
}

func (connection faultConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare is not used by this test")
}

func (connection faultConn) Begin() (driver.Tx, error) {
	return nil, errors.New("transactions are not used by this test")
}

func (connection faultConn) Close() error {
	return nil
}

func (connection faultConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return connection.rows, nil
}

type faultRows struct {
	phase        string
	failure      error
	closed       atomic.Bool
	columnReads  int
	returnedRows int
}

func (rows *faultRows) Columns() []string {
	rows.columnReads++
	// 学習用: 列数を途中で変える壊れたドライバを模擬し、Scanの失敗経路を通します。
	if rows.phase == "scan row" && rows.columnReads > 1 {
		return []string{"id", "unexpected"}
	}
	return []string{"id"}
}

func (rows *faultRows) Close() error {
	rows.closed.Store(true)
	if rows.phase == "close rows" {
		return rows.failure
	}
	return nil
}

func (rows *faultRows) Next(values []driver.Value) error {
	if rows.phase == "read rows" && rows.returnedRows == 1 {
		return rows.failure
	}
	if rows.returnedRows == 2 {
		return io.EOF
	}
	rows.returnedRows++
	values[0] = int64(rows.returnedRows)
	return nil
}
