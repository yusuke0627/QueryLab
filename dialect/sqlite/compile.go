package sqlite

import (
	"strings"

	q "github.com/yusuke0627/QueryLab"
)

// 学習用: SQLと値を分離します。SQL中の?とArgsの要素が、左から順に対応します。
type Statement struct {
	SQL  string
	Args []any
}

// 学習用: CompileはDB実行ではなく、共通モデルからSQLと引数を作る処理です。
func Compile(query q.Select) (Statement, error) {
	// 学習用: 先に全体を検証し、不正なら部分的なSQLを返さずに終了します。
	if err := q.Validate(query); err != nil {
		return Statement{}, err
	}
	// 学習用: 引数の保存先は呼び出しごとに作り、前のクエリの値を持ち越しません。
	compiler := compiler{args: make([]any, 0)}
	// 学習用: Builderは句を順番に書き足すためのバッファです。
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
	sql.WriteString(" FROM " + tableSQL(query.From))
	for _, join := range query.Joins {
		// 学習用: SQLに現れる順に式を処理するため、ONの値はWHEREの値より先に入ります。
		sql.WriteString(" " + string(join.Kind) + " JOIN " + tableSQL(join.Table))
		sql.WriteString(" ON " + compiler.expression(join.On))
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
		// 学習用: SQLiteでOFFSETだけを使うため、件数制限なしを表すLIMIT -1を補います。
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

// 学習用: ポインタレシーバーを使い、同じcompilerのargsへ値を追加します。
// anyは異なる型の値を受け取れます。SQL側には値を埋め込まず?だけを返します。
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
		// 学習用: Scalarの種別に応じて元のGoの値を取り出します。
		// NULLならvalueは初期値のnilのままです。Bool(true)ならboolのtrueになります。
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
		// 学習用: 左、右の順にたどることで、SQLの?と引数の順序を一致させます。
		left := compiler.expression(expression.Args[0])
		right := compiler.expression(expression.Args[1])
		return "(" + left + " " + string(expression.Operator) + " " + right + ")"
	case q.AndExpr, q.OrExpr:
		// 学習用: 子をSQLへ変換してAND/ORで結び、括弧で木のまとまりを維持します。
		// 子の文字列を親で結合し直すため、深い木ではコピーが増える点に注意します。
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
	// 学習用: ここに来ない前提をValidateが支えます。種別追加時は両方の更新が必要です。
	panic("unreachable: expression must be validated")
}

// 学習用: 列名などは値の?では置き換えられないため、識別子として引用します。
// 名前に含まれる二重引用符を二重にしてから、名前全体を二重引用符で囲みます。
func quote(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func tableSQL(table q.Table) string {
	name := quote(table.Name)
	if table.Alias != "" {
		name += " AS " + quote(table.Alias)
	}
	return name
}
