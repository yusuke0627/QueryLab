package gogen

import (
	"fmt"
	"go/format"
	"go/token"
	"strconv"
	"strings"

	q "github.com/yusuke0627/QueryLab"
)

func Generate(query q.Select, packageName string) ([]byte, error) {
	if !token.IsIdentifier(packageName) || packageName == "_" {
		return nil, fmt.Errorf("invalid package name %q", packageName)
	}
	if err := q.Validate(query); err != nil {
		return nil, err
	}
	var source strings.Builder
	fmt.Fprintf(&source, "package %s\n\nimport q %q\n\nvar Query = q.Select{\n", packageName, "github.com/yusuke0627/QueryLab")
	fmt.Fprintf(&source, "From: q.Table{Name: %q, Alias: %q},\n", query.From.Name, query.From.Alias)
	source.WriteString("Columns: []q.Projection{\n")
	for _, projection := range query.Columns {
		fmt.Fprintf(&source, "{Expr: %s, Alias: %q},\n", expression(projection.Expr), projection.Alias)
	}
	source.WriteString("},\n")
	if query.Where != nil {
		fmt.Fprintf(&source, "Where: q.Ptr(%s),\n", expression(*query.Where))
	}
	if len(query.OrderBy) > 0 {
		source.WriteString("OrderBy: []q.Order{\n")
		for _, order := range query.OrderBy {
			fmt.Fprintf(&source, "{Expr: %s, Direction: q.Direction(%q)},\n", expression(order.Expr), order.Direction)
		}
		source.WriteString("},\n")
	}
	if query.Limit != nil {
		fmt.Fprintf(&source, "Limit: q.Ptr(int64(%d)),\n", *query.Limit)
	}
	if query.Offset != nil {
		fmt.Fprintf(&source, "Offset: q.Ptr(int64(%d)),\n", *query.Offset)
	}
	if query.Distinct {
		source.WriteString("Distinct: true,\n")
	}
	source.WriteString("}\n")
	return format.Source([]byte(source.String()))
}

func expression(value q.Expr) string {
	switch value.Kind {
	case q.ColumnExpr:
		if value.Qualifier != "" {
			return fmt.Sprintf("q.Ref(%q, %q)", value.Qualifier, value.Name)
		}
		return fmt.Sprintf("q.Col(%q)", value.Name)
	case q.StarExpr:
		return "q.Star()"
	case q.ValueExpr:
		switch value.Value.Kind {
		case q.NullValue:
			return "q.Null()"
		case q.TextValue:
			return fmt.Sprintf("q.Text(%q)", value.Value.Text)
		case q.IntegerValue:
			return fmt.Sprintf("q.Int(%d)", value.Value.Integer)
		case q.RealValue:
			return "q.Real(" + strconv.FormatFloat(value.Value.Real, 'g', -1, 64) + ")"
		case q.BooleanValue:
			return fmt.Sprintf("q.Bool(%t)", value.Value.Boolean)
		}
	case q.CompareExpr:
		return fmt.Sprintf("q.Compare(q.Operator(%q), %s, %s)", value.Operator, expression(value.Args[0]), expression(value.Args[1]))
	case q.AndExpr, q.OrExpr:
		parts := make([]string, len(value.Args))
		for index, argument := range value.Args {
			parts[index] = expression(argument)
		}
		name := "And"
		if value.Kind == q.OrExpr {
			name = "Or"
		}
		return "q." + name + "(" + strings.Join(parts, ", ") + ")"
	case q.NotExpr:
		return "q.Not(" + expression(value.Args[0]) + ")"
	case q.IsNullExpr:
		return "q.IsNull(" + expression(value.Args[0]) + ")"
	case q.IsNotNullExpr:
		return "q.IsNotNull(" + expression(value.Args[0]) + ")"
	}
	panic("unreachable: expression must be validated")
}
