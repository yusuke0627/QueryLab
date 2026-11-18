package resultmap_test

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"

	q "github.com/yusuke0627/QueryLab"
	"github.com/yusuke0627/QueryLab/execution"
	"github.com/yusuke0627/QueryLab/resultmap"
	_ "github.com/yusuke0627/QueryLab/storage/sqlite"
)

func TestNestLeftJoin(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`
		CREATE TABLE orders (id INTEGER PRIMARY KEY);
		CREATE TABLE items (id INTEGER PRIMARY KEY, order_id INTEGER, product TEXT);
		INSERT INTO orders VALUES (1), (2);
		INSERT INTO items VALUES (3, 1, 'apple'), (4, 1, 'apple');
	`); err != nil {
		t.Fatal(err)
	}
	query := q.Select{
		From: q.Table{Name: "orders", Alias: "o"},
		Joins: []q.Join{{
			Kind: q.LeftJoin, Table: q.Table{Name: "items", Alias: "i"},
			On: q.Eq(q.Ref("o", "id"), q.Ref("i", "order_id")),
		}},
		Columns: []q.Projection{{Expr: q.Ref("o", "id")}, {Expr: q.Ref("i", "id")}, {Expr: q.Ref("i", "product")}},
		OrderBy: []q.Order{{Expr: q.Ref("o", "id")}, {Expr: q.Ref("i", "id")}},
	}
	flat, err := execution.Execute(context.Background(), db, query, execution.Options{})
	if err != nil {
		t.Fatal(err)
	}
	spec := resultmap.Spec{
		Parent:      resultmap.Selection{Key: []int{0}, Values: []int{0}},
		Child:       resultmap.Selection{Key: []int{1}, Values: []int{1, 2}},
		Cardinality: resultmap.OneToMany,
	}
	groups, err := resultmap.Nest(flat, spec)
	want := []resultmap.Group{
		{Parent: []any{int64(1)}, Children: [][]any{{int64(3), "apple"}, {int64(4), "apple"}}},
		{Parent: []any{int64(2)}, Children: [][]any{}},
	}
	if err != nil || !reflect.DeepEqual(groups, want) {
		t.Fatalf("Nest = %#v, %v; want %#v", groups, err, want)
	}
	if len(flat.Rows) != 3 || flat.Columns[0].Name != flat.Columns[1].Name {
		t.Fatalf("expected flat rows with duplicate column names: %#v", flat)
	}
	spec.Cardinality = resultmap.OneToOne
	if _, err := resultmap.Nest(flat, spec); err == nil {
		t.Fatal("one-to-one accepted two different children")
	}
	query.Where = q.Ptr(q.Or(q.Eq(q.Ref("i", "id"), q.Int(3)), q.IsNull(q.Ref("i", "id"))))
	flat, err = execution.Execute(context.Background(), db, query, execution.Options{})
	if err != nil {
		t.Fatal(err)
	}
	groups, err = resultmap.Nest(flat, spec)
	want[0].Children = want[0].Children[:1]
	if err != nil || !reflect.DeepEqual(groups, want) {
		t.Fatalf("one-to-one = %#v, %v; want %#v", groups, err, want)
	}
}

func TestNestCompositeKeysAndFirstSeenOrder(t *testing.T) {
	data := []byte{7}
	flat := execution.Result{Columns: make([]execution.Column, 5), Rows: [][]any{
		{"a:b", "c", int64(1), "child", data},
		{"a", "b:c", int64(1), "child", []byte{8}},
		{"a:b", "c", "1", "child", []byte{9}},
		{"a:b", "c", int64(1), "child", []byte{7}},
	}}
	spec := resultmap.Spec{
		Parent:      resultmap.Selection{Key: []int{0, 1}, Values: []int{0, 1}},
		Child:       resultmap.Selection{Key: []int{2, 3}, Values: []int{2, 4}},
		Cardinality: resultmap.OneToMany,
	}
	groups, err := resultmap.Nest(flat, spec)
	want := []resultmap.Group{
		{Parent: []any{"a:b", "c"}, Children: [][]any{{int64(1), []byte{7}}, {"1", []byte{9}}}},
		{Parent: []any{"a", "b:c"}, Children: [][]any{{int64(1), []byte{8}}}},
	}
	if err != nil || !reflect.DeepEqual(groups, want) {
		t.Fatalf("Nest = %#v, %v; want %#v", groups, err, want)
	}
	groups[0].Children[0][1].([]byte)[0] = 0
	if data[0] != 7 {
		t.Fatal("mapped BLOB shares storage with the flat input")
	}
}

func TestNestRejectsInvalidInputWithoutPartialGroups(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*execution.Result, *resultmap.Spec)
		want string
	}{
		{"truncated", func(result *execution.Result, _ *resultmap.Spec) { result.Truncated = true }, "truncated"},
		{"shape", func(result *execution.Result, _ *resultmap.Spec) { result.Rows[1] = []any{int64(1)} }, "got 1 values"},
		{"bad index", func(_ *execution.Result, spec *resultmap.Spec) { spec.Parent.Key = []int{5} }, "column index"},
		{"null parent", func(result *execution.Result, _ *resultmap.Spec) { result.Rows[1][0] = nil }, "parent key is NULL"},
		{"partial child", func(result *execution.Result, _ *resultmap.Spec) { result.Rows[1][2] = nil }, "partially NULL"},
		{"key type", func(result *execution.Result, _ *resultmap.Spec) { result.Rows[1][2] = 1.5 }, "expected int64 or string"},
		{"parent conflict", func(result *execution.Result, _ *resultmap.Spec) { result.Rows[1][1] = "changed" }, "conflicting parent"},
		{"child conflict", func(result *execution.Result, _ *resultmap.Spec) { result.Rows[1][4] = "changed" }, "conflicting child"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := execution.Result{Columns: make([]execution.Column, 5), Rows: [][]any{
				{int64(1), "parent", int64(2), "part", "child"},
				{int64(1), "parent", int64(2), "part", "child"},
			}}
			spec := resultmap.Spec{
				Parent:      resultmap.Selection{Key: []int{0}, Values: []int{1}},
				Child:       resultmap.Selection{Key: []int{2, 3}, Values: []int{4}},
				Cardinality: resultmap.OneToMany,
			}
			test.edit(&result, &spec)
			if groups, err := resultmap.Nest(result, spec); groups != nil || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Nest = %#v, %v; want error containing %q", groups, err, test.want)
			}
		})
	}
}
