package sqlite_test

import (
	"reflect"
	"strings"
	"testing"

	q "github.com/yusuke0627/QueryLab"
	"github.com/yusuke0627/QueryLab/dialect/sqlite"
)

func TestCompileJoinsAndBindInSQLOrder(t *testing.T) {
	query := q.Select{
		From:    q.Table{Name: "orders", Alias: "o"},
		Columns: []q.Projection{{Expr: q.Text("label")}},
		Joins: []q.Join{
			{Kind: q.InnerJoin, Table: q.Table{Name: "customer\"records", Alias: "c"}, On: q.Eq(q.Ref("c", "name"), q.Text("Alice"))},
			{Kind: q.LeftJoin, Table: q.Table{Name: "items"}, On: q.And(q.Eq(q.Ref("items", "order_id"), q.Ref("o", "id")), q.Eq(q.Ref("c", "active"), q.Bool(true)))},
		},
		Where: q.Ptr(q.Eq(q.Ref("o", "id"), q.Int(7))),
		Limit: q.Ptr(int64(3)),
	}
	statement, err := sqlite.Compile(query)
	wantSQL := `SELECT ? FROM "orders" AS "o" INNER JOIN "customer""records" AS "c" ON ("c"."name" = ?) LEFT JOIN "items" ON (("items"."order_id" = "o"."id") AND ("c"."active" = ?)) WHERE ("o"."id" = ?) LIMIT ?`
	if err != nil || statement.SQL != wantSQL || !reflect.DeepEqual(statement.Args, []any{"label", "Alice", true, int64(7), int64(3)}) {
		t.Fatalf("compile = %#v, %v", statement, err)
	}
}

func TestRejectInvalidJoin(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*q.Select)
		want   string
	}{
		{"kind", func(query *q.Select) { query.Joins[0].Kind = "CROSS" }, "unsupported join kind"},
		{"empty table", func(query *q.Select) { query.Joins[0].Table.Name = "" }, "table:"},
		{"no condition", func(query *q.Select) { query.Joins[0].On = q.Expr{} }, "on:"},
		{"star", func(query *q.Select) { query.Joins[0].On = q.Star() }, "on:"},
		{"duplicate alias", func(query *q.Select) { query.Joins[0].Table.Alias = "O" }, "duplicate table qualifier"},
		{"forward reference", func(query *q.Select) { query.Joins[0].On = q.Eq(q.Ref("later", "id"), q.Ref("o", "id")) }, "unknown table qualifier"},
		{"hidden original name", func(query *q.Select) { query.Columns[0].Expr = q.Ref("orders", "id") }, "unknown table qualifier"},
	} {
		t.Run(test.name, func(t *testing.T) {
			query := q.Select{
				From:    q.Table{Name: "orders", Alias: "o"},
				Columns: []q.Projection{{Expr: q.Ref("O", "id")}},
				Joins: []q.Join{
					{Kind: q.InnerJoin, Table: q.Table{Name: "orders", Alias: "earlier"}, On: q.Eq(q.Ref("O", "id"), q.Ref("earlier", "id"))},
					{Kind: q.LeftJoin, Table: q.Table{Name: "orders", Alias: "later"}, On: q.Eq(q.Ref("earlier", "id"), q.Ref("later", "id"))},
				},
			}
			if _, err := sqlite.Compile(query); err != nil {
				t.Fatalf("valid self-join rejected: %v", err)
			}
			test.change(&query)
			if _, err := sqlite.Compile(query); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("compile error = %v; want %q", err, test.want)
			}
		})
	}
}
