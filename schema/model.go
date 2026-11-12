package schema

// Snapshot は main スキーマのメタデータです。テーブルの行データは含みません。
type Snapshot struct {
	Tables []Table
}

type Table struct {
	Name        string
	Kind        string
	Columns     []Column
	ForeignKeys []ForeignKey
	Indexes     []Index
}

type Column struct {
	Name              string
	DeclaredType      string
	NotNull           bool
	DefaultSQL        *string
	PrimaryKeyOrdinal int
	HiddenCode        int
}

type ForeignKey struct {
	ID              int
	ReferencedTable string
	OnUpdate        string
	OnDelete        string
	Columns         []ForeignKeyColumn
}

type ForeignKeyColumn struct {
	From string
	To   *string
}

type Index struct {
	Name    string
	Unique  bool
	Origin  string
	Partial bool
	Terms   []IndexTerm
}

type IndexTerm struct {
	ColumnID   int
	ColumnName *string
	Descending bool
	Collation  string
}
