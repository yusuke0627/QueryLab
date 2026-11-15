package querylab

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// 学習用: 再帰が深くなりすぎないように制限します。横に広い木の総量は制限しません。
const MaxExpressionDepth = 128

// 学習用: モデルの構造を検証します。DBを見ないので、列の実在までは判定できません。
func Validate(query Select) error {
	sources := make(map[string]struct{}, len(query.Joins)+1)
	if err := addSource(sources, query.From); err != nil {
		return fmt.Errorf("from: %w", err)
	}
	for index, join := range query.Joins {
		if join.Kind != InnerJoin && join.Kind != LeftJoin {
			return fmt.Errorf("joins[%d]: unsupported join kind %q", index, join.Kind)
		}
		if err := addSource(sources, join.Table); err != nil {
			return fmt.Errorf("joins[%d] table: %w", index, err)
		}
		// 学習用: この時点で登録済みなのはFROMと現在までのJOINだけです。
		if err := validateScopedExpr(join.On, false, sources); err != nil {
			return fmt.Errorf("joins[%d] on: %w", index, err)
		}
	}
	if len(query.Columns) == 0 {
		return fmt.Errorf("columns: at least one projection is required")
	}
	for index, projection := range query.Columns {
		if err := validateScopedExpr(projection.Expr, true, sources); err != nil {
			// 学習用: %wで元のエラーを包み、何番目の列で失敗したかを付け加えます。
			return fmt.Errorf("columns[%d]: %w", index, err)
		}
		if projection.Alias != "" {
			if projection.Expr.Kind == StarExpr {
				return fmt.Errorf("columns[%d]: star cannot have an alias", index)
			}
			if err := identifier(projection.Alias); err != nil {
				return fmt.Errorf("columns[%d] alias: %w", index, err)
			}
		}
	}
	if query.Where != nil {
		if err := validateScopedExpr(*query.Where, false, sources); err != nil {
			return fmt.Errorf("where: %w", err)
		}
	}
	for index, order := range query.OrderBy {
		if err := validateScopedExpr(order.Expr, false, sources); err != nil {
			return fmt.Errorf("order_by[%d]: %w", index, err)
		}
		if order.Direction != "" && order.Direction != Ascending && order.Direction != Descending {
			return fmt.Errorf("order_by[%d]: unsupported direction %q", index, order.Direction)
		}
	}
	if query.Limit != nil && *query.Limit < 0 {
		return fmt.Errorf("limit: must be nonnegative")
	}
	if query.Offset != nil && *query.Offset < 0 {
		return fmt.Errorf("offset: must be nonnegative")
	}
	return nil
}

func addSource(sources map[string]struct{}, table Table) error {
	if err := identifier(table.Name); err != nil {
		return err
	}
	name := table.Name
	if table.Alias != "" {
		if err := identifier(table.Alias); err != nil {
			return fmt.Errorf("alias: %w", err)
		}
		name = table.Alias
	}
	key := sourceKey(name)
	if _, exists := sources[key]; exists {
		return fmt.Errorf("duplicate table qualifier %q", name)
	}
	sources[key] = struct{}{}
	return nil
}

func sourceKey(name string) string {
	// 学習用: SQLiteの識別子比較に合わせ、ASCIIの大文字だけを小文字にします。
	return strings.Map(func(character rune) rune {
		if character >= 'A' && character <= 'Z' {
			return character + ('a' - 'A')
		}
		return character
	}, name)
}

func validateScopedExpr(expression Expr, allowStar bool, sources map[string]struct{}) error {
	if err := validateExpr(expression, 0, allowStar); err != nil {
		return err
	}
	return validateQualifiers(expression, sources)
}

func validateQualifiers(expression Expr, sources map[string]struct{}) error {
	if expression.Kind == ColumnExpr && expression.Qualifier != "" {
		if _, exists := sources[sourceKey(expression.Qualifier)]; !exists {
			return fmt.Errorf("unknown table qualifier %q", expression.Qualifier)
		}
	}
	for _, child := range expression.Args {
		if err := validateQualifiers(child, sources); err != nil {
			return err
		}
	}
	return nil
}

func identifier(name string) error {
	if name == "" || strings.ContainsRune(name, 0) || !utf8.ValidString(name) {
		return fmt.Errorf("identifier must be nonempty UTF-8 without NUL")
	}
	return nil
}

func validateExpr(expression Expr, depth int, allowStar bool) error {
	// 学習用: 子を検証するたびにdepthを増やし、深すぎる式や循環をここで止めます。
	if depth >= MaxExpressionDepth {
		return fmt.Errorf("expression exceeds maximum depth %d", MaxExpressionDepth)
	}
	// 学習用: 構造体では「値なのに列名も入っている」データも作れてしまいます。
	// 種別に合わないフィールドを無視せず、不正なモデルとして拒否します。
	if expression.Kind != ColumnExpr && (expression.Name != "" || expression.Qualifier != "") {
		return fmt.Errorf("only column expressions may carry identifiers")
	}
	if expression.Kind != ValueExpr && expression.Value != (Scalar{}) {
		return fmt.Errorf("only value expressions may carry a scalar")
	}
	if expression.Kind != CompareExpr && expression.Operator != "" {
		return fmt.Errorf("only comparison expressions may carry an operator")
	}
	expectedArgs := 0
	switch expression.Kind {
	case ColumnExpr:
		if err := identifier(expression.Name); err != nil {
			return err
		}
		if expression.Qualifier != "" {
			if err := identifier(expression.Qualifier); err != nil {
				return err
			}
		}
	case ValueExpr:
		if err := validateScalar(expression.Value); err != nil {
			return err
		}
	case StarExpr:
		if !allowStar {
			return fmt.Errorf("star is only supported as a complete projection")
		}
	case CompareExpr:
		expectedArgs = 2
		switch expression.Operator {
		case Equal, NotEqual, Greater, GreaterEqual, Less, LessEqual, Matches:
		default:
			return fmt.Errorf("unsupported comparison operator %q", expression.Operator)
		}
		for _, argument := range expression.Args {
			if argument.Kind == ValueExpr && argument.Value.Kind == NullValue {
				return fmt.Errorf("NULL comparisons require IsNull or IsNotNull")
			}
		}
	case AndExpr, OrExpr:
		expectedArgs = len(expression.Args)
		if expectedArgs < 2 {
			return fmt.Errorf("%s requires at least two expressions", expression.Kind)
		}
	case NotExpr, IsNullExpr, IsNotNullExpr:
		expectedArgs = 1
	default:
		return fmt.Errorf("unsupported expression kind %q", expression.Kind)
	}
	if len(expression.Args) != expectedArgs {
		return fmt.Errorf("%s expects %d arguments, got %d", expression.Kind, expectedArgs, len(expression.Args))
	}
	// 学習用: 親の形を検査した後で子へ進む、深さ優先の再帰処理です。
	// エラーには子の位置も付け、木のどこが不正かを追えるようにします。
	for index, argument := range expression.Args {
		if err := validateExpr(argument, depth+1, false); err != nil {
			return fmt.Errorf("args[%d]: %w", index, err)
		}
	}
	return nil
}

func validateScalar(value Scalar) error {
	// 学習用: 種別と対応する値だけを持つ「正しい形」を作り、最後に入力と比較します。
	// これにより、整数のScalarに文字列の値も入っている、といった混在を検出します。
	canonical := Scalar{Kind: value.Kind}
	switch value.Kind {
	case NullValue:
	case TextValue:
		if !utf8.ValidString(value.Text) {
			return fmt.Errorf("text must be valid UTF-8")
		}
		canonical.Text = value.Text
	case IntegerValue:
		canonical.Integer = value.Integer
	case RealValue:
		if math.IsNaN(value.Real) || math.IsInf(value.Real, 0) {
			return fmt.Errorf("real must be finite")
		}
		canonical.Real = value.Real
	case BooleanValue:
		canonical.Boolean = value.Boolean
	default:
		return fmt.Errorf("unsupported scalar kind %q", value.Kind)
	}
	if value != canonical {
		return fmt.Errorf("scalar carries fields incompatible with kind %q", value.Kind)
	}
	return nil
}
