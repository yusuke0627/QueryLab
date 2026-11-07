package querylab

type Select struct {
	From     Table        `json:"from"`
	Columns  []Projection `json:"columns"`
	Where    *Expr        `json:"where,omitempty"`
	OrderBy  []Order      `json:"order_by,omitempty"`
	Limit    *int64       `json:"limit,omitempty"`
	Offset   *int64       `json:"offset,omitempty"`
	Distinct bool         `json:"distinct,omitempty"`
}

type Table struct {
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
}

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

type Scalar struct {
	Kind    ScalarKind `json:"kind,omitempty"`
	Text    string     `json:"text,omitempty"`
	Integer int64      `json:"integer,omitempty"`
	Real    float64    `json:"real,omitempty"`
	Boolean bool       `json:"boolean,omitempty"`
}
