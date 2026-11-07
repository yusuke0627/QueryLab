package querylab

// 学習用: Select は「どんな検索をしたいか」を保存するデータです。
// この構造体を作るだけではSQLの生成やDBへの問い合わせは行いません。
type Select struct {
	From    Table        `json:"from"`
	Columns []Projection `json:"columns"`
	// 学習用: *Expr は式へのポインタです。nilならWHEREが未指定だと表せます。
	Where   *Expr   `json:"where,omitempty"`
	OrderBy []Order `json:"order_by,omitempty"`
	// 学習用: LIMITはnilなら制限なし、0へのポインタなら0行です。
	// int64だけだと未指定と0を区別できないため、ポインタを使います。
	Limit    *int64 `json:"limit,omitempty"`
	Offset   *int64 `json:"offset,omitempty"`
	Distinct bool   `json:"distinct,omitempty"`
}

// 学習用: Alias はSQLの別名です。Nameがusers、Aliasがuなら users AS u を表します。
type Table struct {
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
}

// 学習用: Projection はSELECTで取り出す項目です。列のほか、値や式も指定できます。
type Projection struct {
	Expr  Expr   `json:"expr"`
	Alias string `json:"alias,omitempty"`
}

type Direction string

const (
	Ascending  Direction = "ASC"
	Descending Direction = "DESC"
)

type Order struct {
	Expr      Expr      `json:"expr"`
	Direction Direction `json:"direction,omitempty"`
}

// 学習用: stringを基に別の型を定義し、式の種類を名前で区別します。
// 定数にない値も作れてしまうため、後からValidateで検査します。
type ExprKind string

const (
	ColumnExpr    ExprKind = "column"
	ValueExpr     ExprKind = "value"
	StarExpr      ExprKind = "star"
	CompareExpr   ExprKind = "compare"
	AndExpr       ExprKind = "and"
	OrExpr        ExprKind = "or"
	NotExpr       ExprKind = "not"
	IsNullExpr    ExprKind = "is_null"
	IsNotNullExpr ExprKind = "is_not_null"
)

type Operator string

const (
	Equal        Operator = "="
	NotEqual     Operator = "<>"
	Greater      Operator = ">"
	GreaterEqual Operator = ">="
	Less         Operator = "<"
	LessEqual    Operator = "<="
	Matches      Operator = "LIKE"
)

// 学習用: Expr は列・値・比較などを共通の型で表す「式のノード」です。
// Kindで種類を選び、Argsに子の式を持たせると木構造になります。
// 例: Eq(Col("active"), Bool(true))は比較を親、列と値を子に持ちます。
type Expr struct {
	Kind      ExprKind `json:"kind"`
	Name      string   `json:"name,omitempty"`
	Qualifier string   `json:"qualifier,omitempty"`
	Value     Scalar   `json:"value,omitempty"`
	Operator  Operator `json:"operator,omitempty"`
	Args      []Expr   `json:"args,omitempty"`
}

type ScalarKind string

const (
	NullValue    ScalarKind = "null"
	TextValue    ScalarKind = "text"
	IntegerValue ScalarKind = "integer"
	RealValue    ScalarKind = "real"
	BooleanValue ScalarKind = "boolean"
)

// 学習用: Scalar は列名ではなく、trueや20などの値そのものを保存します。
// Kindに対応したフィールドだけを使います。Bool(true)ならBooleanがtrueです。
// 型を明示することでJSONへの保存・復元後も整数と実数などを区別できます。
type Scalar struct {
	Kind    ScalarKind `json:"kind,omitempty"`
	Text    string     `json:"text,omitempty"`
	Integer int64      `json:"integer,omitempty"`
	Real    float64    `json:"real,omitempty"`
	Boolean bool       `json:"boolean,omitempty"`
}
