package sqlite

import (
	"strings"

	q "github.com/yusuke0627/QueryLab"
)

type Statement struct {
	SQL  string
	Args []any
}

func Compile(query q.Select) (Statement, error) {
	if err := q.Validate(query); err != nil {
		return Statement{}, err
	}
	compiler := compiler{args: make([]any, 0)}
	var sql strings.Builder
	sql.WriteString("SELECT ")
	if query.Distinct {
		sql.WriteString("DISTINCT ")
	}
	for index, projection := range query.Columns {
		if index > 0 {
			sql.WriteString(", ")
		}
		sql.WriteString(compiler.expression(projection.Expr))
		if projection.Alias != "" {
			sql.WriteString(" AS " + quote(projection.Alias))
		}
	}
	sql.WriteString(" FROM " + quote(query.From.Name))
	if query.From.Alias != "" {
		sql.WriteString(" AS " + quote(query.From.Alias))
	}
	if query.Where != nil {
		sql.WriteString(" WHERE " + compiler.expression(*query.Where))
	}
	if len(query.OrderBy) > 0 {
		sql.WriteString(" ORDER BY ")
		for index, order := range query.OrderBy {
			if index > 0 {
				sql.WriteString(", ")
			}
			sql.WriteString(compiler.expression(order.Expr))
			if order.Direction != "" {
				sql.WriteString(" " + string(order.Direction))
			}
		}
	}
	if query.Limit != nil {
		sql.WriteString(" LIMIT " + compiler.bind(*query.Limit))
	} else if query.Offset != nil {
		sql.WriteString(" LIMIT -1")
	}
	if query.Offset != nil {
		sql.WriteString(" OFFSET " + compiler.bind(*query.Offset))
	}
	return Statement{SQL: sql.String(), Args: compiler.args}, nil
}

type compiler struct {
	args []any
}

func (compiler *compiler) bind(value any) string {
	compiler.args = append(compiler.args, value)
	return "?"
}

func (compiler *compiler) expression(expression q.Expr) string {
	switch expression.Kind {
	case q.ColumnExpr:
		if expression.Qualifier != "" {
			return quote(expression.Qualifier) + "." + quote(expression.Name)
		}
		return quote(expression.Name)
	case q.StarExpr:
		return "*"
	case q.ValueExpr:
		var value any
		switch expression.Value.Kind {
		case q.TextValue:
			value = expression.Value.Text
		case q.IntegerValue:
			value = expression.Value.Integer
		case q.RealValue:
			value = expression.Value.Real
		case q.BooleanValue:
			value = expression.Value.Boolean
		}
		return compiler.bind(value)
	case q.CompareExpr:
		left := compiler.expression(expression.Args[0])
		right := compiler.expression(expression.Args[1])
		return "(" + left + " " + string(expression.Operator) + " " + right + ")"
	case q.AndExpr, q.OrExpr:
		parts := make([]string, len(expression.Args))
		for index, argument := range expression.Args {
			parts[index] = compiler.expression(argument)
		}
		operator := " AND "
		if expression.Kind == q.OrExpr {
			operator = " OR "
		}
		return "(" + strings.Join(parts, operator) + ")"
	case q.NotExpr:
		return "(NOT " + compiler.expression(expression.Args[0]) + ")"
	case q.IsNullExpr:
		return "(" + compiler.expression(expression.Args[0]) + " IS NULL)"
	case q.IsNotNullExpr:
		return "(" + compiler.expression(expression.Args[0]) + " IS NOT NULL)"
	}
	panic("unreachable: expression must be validated")
}

func quote(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
