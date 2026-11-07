package querylab_test

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	q "github.com/yusuke0627/QueryLab"
)

func baseQuery() q.Select {
	return q.Select{From: q.Table{Name: "users"}, Columns: []q.Projection{{Expr: q.Star()}}}
}

func TestValidateRejectsMalformedModels(t *testing.T) {
	tests := []struct {
		name   string
		change func(*q.Select)
		want   string
	}{
		{"missing table", func(query *q.Select) { query.From.Name = "" }, "from"},
		{"NUL identifier", func(query *q.Select) { query.From.Name = "users\x00" }, "identifier"},
		{"invalid UTF8", func(query *q.Select) { query.From.Name = "\xff" }, "identifier"},
		{"empty projection", func(query *q.Select) { query.Columns = nil }, "columns"},
		{"star alias", func(query *q.Select) { query.Columns[0].Alias = "all" }, "star"},
		{"star predicate", func(query *q.Select) { query.Where = q.Ptr(q.Star()) }, "star"},
		{"star order", func(query *q.Select) { query.OrderBy = []q.Order{{Expr: q.Star()}} }, "star"},
		{"unknown kind", func(query *q.Select) { query.Where = q.Ptr(q.Expr{Kind: "raw"}) }, "kind"},
		{"empty and", func(query *q.Select) { query.Where = q.Ptr(q.And()) }, "at least two"},
		{"single or", func(query *q.Select) { query.Where = q.Ptr(q.Or(q.Bool(true))) }, "at least two"},
		{"missing operand", func(query *q.Select) { query.Where = q.Ptr(q.Expr{Kind: q.NotExpr}) }, "expects 1"},
		{"operator injection", func(query *q.Select) { query.Where = q.Ptr(q.Compare("= 1; --", q.Int(1), q.Int(1))) }, "operator"},
		{"NULL equality", func(query *q.Select) { query.Where = q.Ptr(q.Eq(q.Col("id"), q.Null())) }, "IsNull"},
		{"nonfinite real", func(query *q.Select) { query.Where = q.Ptr(q.Real(math.Inf(1))) }, "finite"},
		{"NaN", func(query *q.Select) { query.Where = q.Ptr(q.Real(math.NaN())) }, "finite"},
		{"invalid text", func(query *q.Select) { query.Where = q.Ptr(q.Text("\xff")) }, "UTF-8"},
		{"unused scalar data", func(query *q.Select) {
			query.Where = q.Ptr(q.Expr{Kind: q.ValueExpr, Value: q.Scalar{Kind: q.NullValue, Text: "hidden"}})
		}, "incompatible"},
		{"unused identifiers", func(query *q.Select) {
			query.Where = q.Ptr(q.Expr{Kind: q.ValueExpr, Name: "hidden", Value: q.Scalar{Kind: q.NullValue}})
		}, "identifiers"},
		{"unused operands", func(query *q.Select) {
			expression := q.Col("id")
			expression.Args = []q.Expr{q.Int(1)}
			query.Where = &expression
		}, "expects 0"},
		{"bad direction", func(query *q.Select) {
			query.OrderBy = []q.Order{{Expr: q.Col("id"), Direction: "DESC; DROP TABLE users"}}
		}, "direction"},
		{"negative limit", func(query *q.Select) { query.Limit = q.Ptr(int64(-1)) }, "limit"},
		{"negative offset", func(query *q.Select) { query.Offset = q.Ptr(int64(-1)) }, "offset"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query := baseQuery()
			test.change(&query)
			if err := q.Validate(query); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Validate() = %v, want error containing %q", err, test.want)
			}
		})
	}
}

func TestExpressionDepthAndCycles(t *testing.T) {
	query := baseQuery()
	expression := q.Bool(true)
	for depth := 0; depth < q.MaxExpressionDepth; depth++ {
		expression = q.Not(expression)
	}
	query.Where = &expression
	if err := q.Validate(query); err == nil || !strings.Contains(err.Error(), "depth") {
		t.Fatalf("Validate() = %v, want depth error", err)
	}
	cycle := q.Not(q.Bool(true))
	cycle.Args[0] = cycle
	query.Where = &cycle
	if err := q.Validate(query); err == nil || !strings.Contains(err.Error(), "depth") {
		t.Fatalf("cyclic Validate() = %v, want depth error", err)
	}
}

func TestJSONPreservesScalarTypes(t *testing.T) {
	query := baseQuery()
	query.Columns = []q.Projection{
		{Expr: q.Int(math.MaxInt64)}, {Expr: q.Int(math.MinInt64)},
		{Expr: q.Text("日本語\n\x00")}, {Expr: q.Real(1.25)},
		{Expr: q.Bool(false)}, {Expr: q.Null()},
	}
	encoded, err := json.Marshal(query)
	if err != nil {
		t.Fatal(err)
	}
	var restored q.Select
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if err := q.Validate(restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(query, restored) {
		t.Fatalf("model changed after JSON roundtrip: %s", encoded)
	}
}
