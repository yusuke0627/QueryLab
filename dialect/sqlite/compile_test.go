package sqlite_test

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	q "github.com/yusuke0627/QueryLab"
	"github.com/yusuke0627/QueryLab/dialect/sqlite"
)

func TestCompile(t *testing.T) {
	tests := []struct {
		name  string
		query q.Select
		sql   string
		args  []any
	}{
		{
			name:  "star",
			query: q.Select{From: q.Table{Name: "users"}, Columns: []q.Projection{{Expr: q.Star()}}},
			sql:   `SELECT * FROM "users"`, args: []any{},
		},
		{
			name: "nested filters and pagination",
			query: q.Select{
				From:    q.Table{Name: "users", Alias: "u"},
				Columns: []q.Projection{{Expr: q.Ref("u", "id"), Alias: "user_id"}},
				Where:   q.Ptr(q.And(q.Compare(q.GreaterEqual, q.Col("age"), q.Int(20)), q.Or(q.Eq(q.Col("city"), q.Text("Tokyo")), q.IsNull(q.Col("city"))), q.Not(q.Eq(q.Col("blocked"), q.Bool(true))))),
				OrderBy: []q.Order{{Expr: q.Col("age"), Direction: q.Descending}, {Expr: q.Col("id")}},
				Limit:   q.Ptr(int64(10)), Offset: q.Ptr(int64(5)), Distinct: true,
			},
			sql:  `SELECT DISTINCT "u"."id" AS "user_id" FROM "users" AS "u" WHERE (("age" >= ?) AND (("city" = ?) OR ("city" IS NULL)) AND (NOT ("blocked" = ?))) ORDER BY "age" DESC, "id" LIMIT ? OFFSET ?`,
			args: []any{int64(20), "Tokyo", true, int64(10), int64(5)},
		},
		{
			name:  "quoted identifiers and bound injection text",
			query: q.Select{From: q.Table{Name: `strange"table`}, Columns: []q.Projection{{Expr: q.Col("select"), Alias: `a"b`}}, Where: q.Ptr(q.Eq(q.Col("name"), q.Text("'; DROP TABLE users; --")))},
			sql:   `SELECT "select" AS "a""b" FROM "strange""table" WHERE ("name" = ?)`, args: []any{"'; DROP TABLE users; --"},
		},
		{
			name:  "offset without limit",
			query: q.Select{From: q.Table{Name: "users"}, Columns: []q.Projection{{Expr: q.Col("id")}}, Offset: q.Ptr(int64(2))},
			sql:   `SELECT "id" FROM "users" LIMIT -1 OFFSET ?`, args: []any{int64(2)},
		},
		{
			name:  "zero limit",
			query: q.Select{From: q.Table{Name: "users"}, Columns: []q.Projection{{Expr: q.Star()}}, Limit: q.Ptr(int64(0))},
			sql:   `SELECT * FROM "users" LIMIT ?`, args: []any{int64(0)},
		},
		{
			name:  "projection parameters precede filter and ordering parameters",
			query: q.Select{From: q.Table{Name: "users"}, Columns: []q.Projection{{Expr: q.Text("tag")}, {Expr: q.Null()}, {Expr: q.Real(1.5)}}, Where: q.Ptr(q.IsNotNull(q.Col("id"))), OrderBy: []q.Order{{Expr: q.Int(7)}}},
			sql:   `SELECT ?, ?, ? FROM "users" WHERE ("id" IS NOT NULL) ORDER BY ?`, args: []any{"tag", nil, 1.5, int64(7)},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before, err := json.Marshal(test.query)
			if err != nil {
				t.Fatal(err)
			}
			got, err := sqlite.Compile(test.query)
			if err != nil {
				t.Fatal(err)
			}
			if got.SQL != test.sql || !reflect.DeepEqual(got.Args, test.args) {
				t.Fatalf("got %s %#v; want %s %#v", got.SQL, got.Args, test.sql, test.args)
			}
			again, err := sqlite.Compile(test.query)
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Fatalf("nondeterministic compile: %v", err)
			}
			after, err := json.Marshal(test.query)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatal("Compile mutated its input")
			}
		})
	}
}

func TestCompileInvalidModelHasNoPartialStatement(t *testing.T) {
	statement, err := sqlite.Compile(q.Select{})
	if err == nil || statement.SQL != "" || statement.Args != nil {
		t.Fatalf("Compile() = %#v, %v", statement, err)
	}
}

func TestGeneratedSQLAgainstSQLite(t *testing.T) {
	binary, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("sqlite3 CLI not installed; pure Go compiler tests still run")
	}
	query := q.Select{
		From: q.Table{Name: "users"}, Columns: []q.Projection{{Expr: q.Col("id")}},
		Where:   q.Ptr(q.And(q.Compare(q.GreaterEqual, q.Col("age"), q.Int(20)), q.Or(q.Eq(q.Col("city"), q.Text("Tokyo")), q.IsNull(q.Col("city"))))),
		OrderBy: []q.Order{{Expr: q.Col("id"), Direction: q.Descending}},
		Offset:  q.Ptr(int64(1)),
	}
	statement, err := sqlite.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(statement.Args, []any{int64(20), "Tokyo", int64(1)}) {
		t.Fatalf("unexpected arguments: %#v", statement.Args)
	}
	script := ".bail on\nCREATE TABLE users (id INTEGER, age INTEGER, city TEXT);\nINSERT INTO users VALUES (1, 30, 'Tokyo'), (2, 10, 'Tokyo'), (3, 22, NULL), (4, 40, 'Osaka');\n.parameter init\n.parameter set ?1 20\n.parameter set ?2 'Tokyo'\n.parameter set ?3 1\n" + statement.SQL + ";\n"
	command := exec.Command(binary, ":memory:")
	command.Stdin = strings.NewReader(script)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("sqlite3: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "1" {
		t.Fatalf("unexpected SQLite rows: %s", output)
	}
}
