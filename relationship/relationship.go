package relationship

import (
	"fmt"
	"strings"

	q "github.com/yusuke0627/QueryLab"
	"github.com/yusuke0627/QueryLab/schema"
)

// Pair は左と右で比較する列の組です。
type Pair struct {
	LeftColumn  string
	RightColumn string
}

// Candidate は外部キー1本に対応する候補です。ForeignKeyIDは外部キーを持つテーブル内での番号です。
type Candidate struct {
	LeftTable        string
	RightTable       string
	ForeignKeyID     int
	ForeignKeyOnLeft bool
	Pairs            []Pair
}

// Between は指定した左右のテーブルを直接つなぐ外部キーを探します。クエリは変更しません。
func Between(snapshot schema.Snapshot, leftName, rightName string) ([]Candidate, error) {
	left := findTable(snapshot, leftName)
	right := findTable(snapshot, rightName)
	if left == nil || right == nil {
		return nil, fmt.Errorf("unknown table in relationship: %q, %q", leftName, rightName)
	}
	candidates := make([]Candidate, 0)
	// 学習用: 外部キーは左にも右にもあり得ます。自己参照でも左右の役割を別々に提示します。
	for _, onLeft := range []bool{true, false} {
		child, parent := left, right
		if !onLeft {
			child, parent = right, left
		}
		for _, key := range child.ForeignKeys {
			if !sameName(key.ReferencedTable, parent.Name) {
				continue
			}
			pairs, err := resolvePairs(child, parent, key, onLeft)
			if err != nil {
				return nil, fmt.Errorf("table %q foreign key %d: %w", child.Name, key.ID, err)
			}
			candidates = append(candidates, Candidate{
				LeftTable: left.Name, RightTable: right.Name, ForeignKeyID: key.ID,
				ForeignKeyOnLeft: onLeft, Pairs: pairs,
			})
		}
	}
	return candidates, nil
}

// Join は選んだ候補を明示的なJOINに変換します。左はクエリに登録済みのテーブルを指定します。
func (candidate Candidate) Join(kind q.JoinKind, left, right q.Table) (q.Join, error) {
	if !sameName(left.Name, candidate.LeftTable) || !sameName(right.Name, candidate.RightTable) {
		return q.Join{}, fmt.Errorf("join tables do not match the selected relationship")
	}
	if len(candidate.Pairs) == 0 {
		return q.Join{}, fmt.Errorf("relationship has no column pairs")
	}
	conditions := make([]q.Expr, 0, len(candidate.Pairs))
	for _, pair := range candidate.Pairs {
		leftColumn := q.Ref(qualifier(left), pair.LeftColumn)
		rightColumn := q.Ref(qualifier(right), pair.RightColumn)
		// 学習用: 両側の照合順序が違う場合に備え、比較式では参照先の列を先に書きます。
		if candidate.ForeignKeyOnLeft {
			leftColumn, rightColumn = rightColumn, leftColumn
		}
		conditions = append(conditions, q.Eq(leftColumn, rightColumn))
	}
	// 学習用: 複合キーは一部だけ一致しても不十分なので、すべての列比較をANDで結びます。
	on := conditions[0]
	if len(conditions) > 1 {
		on = q.And(conditions...)
	}
	join := q.Join{Kind: kind, Table: right, On: on}
	if err := q.Validate(q.Select{From: left, Columns: []q.Projection{{Expr: q.Star()}}, Joins: []q.Join{join}}); err != nil {
		return q.Join{}, err
	}
	return join, nil
}

func resolvePairs(child, parent *schema.Table, key schema.ForeignKey, onLeft bool) ([]Pair, error) {
	if len(key.Columns) == 0 {
		return nil, fmt.Errorf("no columns")
	}
	implicit := key.Columns[0].To == nil
	var primary []string
	if implicit {
		primary = make([]string, len(key.Columns))
		count := 0
		for _, column := range parent.Columns {
			ordinal := column.PrimaryKeyOrdinal
			if ordinal == 0 {
				continue
			}
			if ordinal < 1 || ordinal > len(primary) || primary[ordinal-1] != "" || column.Name == "" {
				return nil, fmt.Errorf("referenced primary key does not match foreign key")
			}
			// 学習用: 列の表示順ではなく、PRIMARY KEYの中の番号で相手の列を並べます。
			primary[ordinal-1] = column.Name
			count++
		}
		if count != len(primary) {
			return nil, fmt.Errorf("referenced primary key does not match foreign key")
		}
	}
	pairs := make([]Pair, 0, len(key.Columns))
	for index, column := range key.Columns {
		if (column.To == nil) != implicit {
			return nil, fmt.Errorf("mixed explicit and implicit referenced columns")
		}
		var target string
		if implicit {
			target = primary[index]
		} else {
			target = *column.To
		}
		from, okFrom := findColumn(child, column.From)
		to, okTo := findColumn(parent, target)
		if !okFrom || !okTo {
			return nil, fmt.Errorf("unknown column pair %q -> %q", column.From, target)
		}
		if onLeft {
			pairs = append(pairs, Pair{LeftColumn: from, RightColumn: to})
		} else {
			pairs = append(pairs, Pair{LeftColumn: to, RightColumn: from})
		}
	}
	return pairs, nil
}

func findTable(snapshot schema.Snapshot, name string) *schema.Table {
	for index := range snapshot.Tables {
		if sameName(snapshot.Tables[index].Name, name) {
			return &snapshot.Tables[index]
		}
	}
	return nil
}

func findColumn(table *schema.Table, name string) (string, bool) {
	for _, column := range table.Columns {
		if sameName(column.Name, name) {
			return column.Name, true
		}
	}
	return "", false
}

func qualifier(table q.Table) string {
	if table.Alias != "" {
		return table.Alias
	}
	return table.Name
}

// 学習用: SQLiteと同じようにASCIIの英字だけ大文字・小文字を同一視します。
func sameName(left, right string) bool {
	fold := func(name string) string {
		return strings.Map(func(character rune) rune {
			if character >= 'A' && character <= 'Z' {
				return character + ('a' - 'A')
			}
			return character
		}, name)
	}
	return fold(left) == fold(right)
}
