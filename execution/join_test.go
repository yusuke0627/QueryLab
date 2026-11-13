package execution_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	q "github.com/yusuke0627/QueryLab"
	"github.com/yusuke0627/QueryLab/execution"
	sqlitedb "github.com/yusuke0627/QueryLab/storage/sqlite"
)

func TestExecuteJoin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orders.db")
	setup, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	_, setupErr := setup.Exec(`
		CREATE TABLE orders (id INTEGER);
		CREATE TABLE items (order_id INTEGER, product TEXT);
		INSERT INTO orders VALUES (1), (2);
		INSERT INTO items VALUES (1, 'apple'), (1, 'bread');
	`)
	if err := errors.Join(setupErr, setup.Close()); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	db, err := sqlitedb.OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	query := q.Select{
		From:    q.Table{Name: "orders", Alias: "o"},
		Columns: []q.Projection{{Expr: q.Ref("o", "id")}, {Expr: q.Ref("i", "product")}},
		Joins: []q.Join{{
			Kind: q.InnerJoin, Table: q.Table{Name: "items", Alias: "i"},
			On: q.Eq(q.Ref("o", "id"), q.Ref("i", "order_id")),
		}},
		OrderBy: []q.Order{{Expr: q.Ref("o", "id")}, {Expr: q.Ref("i", "product")}},
	}
	// 学習用: 注文1に明細が2件あります。注文2はLEFTのときだけ残ります。
	for _, test := range []struct {
		kind q.JoinKind
		want [][]any
	}{
		{q.InnerJoin, [][]any{{int64(1), "apple"}, {int64(1), "bread"}}},
		{q.LeftJoin, [][]any{{int64(1), "apple"}, {int64(1), "bread"}, {int64(2), nil}}},
	} {
		query.Joins[0].Kind = test.kind
		result, err := execution.Execute(ctx, db, query, execution.Options{})
		if err != nil || !reflect.DeepEqual(result.Rows, test.want) {
			t.Fatalf("%s JOIN: %#v, %v; want %#v", test.kind, result.Rows, err, test.want)
		}
	}

	filter := q.Eq(q.Ref("i", "product"), q.Text("apple"))
	query.Joins[0].On = q.And(query.Joins[0].On, filter)
	result, err := execution.Execute(ctx, db, query, execution.Options{})
	if err != nil || !reflect.DeepEqual(result.Rows, [][]any{{int64(1), "apple"}, {int64(2), nil}}) {
		t.Fatalf("filter in ON: %#v, %v", result.Rows, err)
	}
	query.Joins[0].On = q.Eq(q.Ref("o", "id"), q.Ref("i", "order_id"))
	query.Where = q.Ptr(filter)
	result, err = execution.Execute(ctx, db, query, execution.Options{})
	if err != nil || !reflect.DeepEqual(result.Rows, [][]any{{int64(1), "apple"}}) {
		t.Fatalf("filter in WHERE: %#v, %v", result.Rows, err)
	}
}
