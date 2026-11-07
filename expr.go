package querylab

func Col(name string) Expr {
	return Expr{Kind: ColumnExpr, Name: name}
}

func Ref(qualifier, name string) Expr {
	return Expr{Kind: ColumnExpr, Name: name, Qualifier: qualifier}
}

func Star() Expr {
	return Expr{Kind: StarExpr}
}

func Text(value string) Expr {
	return Expr{Kind: ValueExpr, Value: Scalar{Kind: TextValue, Text: value}}
}

func Int(value int64) Expr {
	return Expr{Kind: ValueExpr, Value: Scalar{Kind: IntegerValue, Integer: value}}
}

func Real(value float64) Expr {
	return Expr{Kind: ValueExpr, Value: Scalar{Kind: RealValue, Real: value}}
}

func Bool(value bool) Expr {
	return Expr{Kind: ValueExpr, Value: Scalar{Kind: BooleanValue, Boolean: value}}
}

func Null() Expr {
	return Expr{Kind: ValueExpr, Value: Scalar{Kind: NullValue}}
}

func Compare(operator Operator, left, right Expr) Expr {
	return Expr{Kind: CompareExpr, Operator: operator, Args: []Expr{left, right}}
}

func Eq(left, right Expr) Expr {
	return Compare(Equal, left, right)
}

func And(expressions ...Expr) Expr {
	return Expr{Kind: AndExpr, Args: append([]Expr(nil), expressions...)}
}

func Or(expressions ...Expr) Expr {
	return Expr{Kind: OrExpr, Args: append([]Expr(nil), expressions...)}
}

func Not(expression Expr) Expr {
	return Expr{Kind: NotExpr, Args: []Expr{expression}}
}

func IsNull(expression Expr) Expr {
	return Expr{Kind: IsNullExpr, Args: []Expr{expression}}
}

func IsNotNull(expression Expr) Expr {
	return Expr{Kind: IsNotNullExpr, Args: []Expr{expression}}
}

func Ptr[Value any](value Value) *Value {
	return &value
}
