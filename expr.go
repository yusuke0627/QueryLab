package querylab

// 学習用: Col は列を表すExprを作ります。列の値をここで取得するわけではありません。
func Col(name string) Expr {
	return Expr{Kind: ColumnExpr, Name: name}
}

// 学習用: Ref("u", "id")は修飾された列u.idを表します。
// Col("u.id")だと、ドットを含む名前の単一の列を表します。
func Ref(qualifier, name string) Expr {
	return Expr{Kind: ColumnExpr, Name: name, Qualifier: qualifier}
}

// 学習用: Star はSELECT *の「全列」を表します。比較の値には使えません。
func Star() Expr {
	return Expr{Kind: StarExpr}
}

// 学習用: Goの文字列を、クエリの中で使う値のExprに包みます。
func Text(value string) Expr {
	return Expr{Kind: ValueExpr, Value: Scalar{Kind: TextValue, Text: value}}
}

// 学習用: Int(20)は整数の値ノードです。SQL生成時には?と引数20に分かれます。
func Int(value int64) Expr {
	return Expr{Kind: ValueExpr, Value: Scalar{Kind: IntegerValue, Integer: value}}
}

// 学習用: Real は小数を含む数値を表します。NaNや無限大は検証で拒否します。
func Real(value float64) Expr {
	return Expr{Kind: ValueExpr, Value: Scalar{Kind: RealValue, Real: value}}
}

// 学習用: Bool(true)はGoのbool値trueを、クエリ用のExprに変換します。
// Eqは両側にExprを受け取るため、trueを直接渡す代わりにこの関数で包みます。
// 条件を判定する関数ではありません。SQL生成時に?と引数trueになります。
func Bool(value bool) Expr {
	return Expr{Kind: ValueExpr, Value: Scalar{Kind: BooleanValue, Boolean: value}}
}

// 学習用: Null はSQLのNULL値です。列のNULL判定にはIsNullやIsNotNullを使います。
func Null() Expr {
	return Expr{Kind: ValueExpr, Value: Scalar{Kind: NullValue}}
}

// 学習用: 比較の種類と左右の式を保存します。Args[0]が左、Args[1]が右です。
func Compare(operator Operator, left, right Expr) Expr {
	return Expr{Kind: CompareExpr, Operator: operator, Args: []Expr{left, right}}
}

// 学習用: Eq(Col("active"), Bool(true))は「active列がtrueと等しい」という条件のデータです。
// Goで今すぐ==を計算するのではなく、後でSQLの比較式に変換します。
func Eq(left, right Expr) Expr {
	return Compare(Equal, left, right)
}

// 学習用: ...Expr は可変長引数で、複数の式をまとめて渡せます。
// expressions...で要素を展開して新しいスライスにコピーし、入力の配列共有を避けます。
// ただし各Exprの内部にあるスライスまで再帰的にコピーするわけではありません。
func And(expressions ...Expr) Expr {
	return Expr{Kind: AndExpr, Args: append([]Expr(nil), expressions...)}
}

// 学習用: Or は「いずれかを満たす」を表す親ノードです。AndもOrも2つ以上の式が必要です。
func Or(expressions ...Expr) Expr {
	return Expr{Kind: OrExpr, Args: append([]Expr(nil), expressions...)}
}

// 学習用: Not は子の条件を否定します。結果のtrue/falseではなく、子の式を保持します。
func Not(expression Expr) Expr {
	return Expr{Kind: NotExpr, Args: []Expr{expression}}
}

// 学習用: IsNull(Col("deleted_at"))は deleted_at IS NULL を表します。
// SQLでは「= NULL」をNULL判定の代わりにできないため、専用の式にしています。
func IsNull(expression Expr) Expr {
	return Expr{Kind: IsNullExpr, Args: []Expr{expression}}
}

// 学習用: IsNotNull は「値がNULLではない」という条件を表します。
func IsNotNull(expression Expr) Expr {
	return Expr{Kind: IsNotNullExpr, Args: []Expr{expression}}
}

// 学習用: [Value any]はジェネリクスの型引数で、任意の型の値を受け取れます。
// &valueは値へのポインタです。Where: Ptr(expr)やLimit: Ptr(int64(20))に使います。
// 戻った後もGoが必要な値の寿命を管理します。参照先を含む深いコピーではありません。
func Ptr[Value any](value Value) *Value {
	return &value
}
