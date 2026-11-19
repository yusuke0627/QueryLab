package main

import (
	"fmt"
	"strconv"
	"strings"

	q "github.com/yusuke0627/QueryLab"
	"github.com/yusuke0627/QueryLab/dialect/sqlite"
	"github.com/yusuke0627/QueryLab/gogen"
)

// 学習用: Draftは画面の入力です。SQLの意味を持つq.Selectとは別のデータにします。
type Draft struct {
	Table      string
	Columns    string
	Limit      string
	ActiveOnly bool
	NotDeleted bool
}

type Preview struct {
	SQL  string
	Args []any
	Go   string
}

func sampleDraft() Draft {
	return Draft{Table: "users", Columns: "id, name", Limit: "20", ActiveOnly: true, NotDeleted: true}
}

// 学習用: 画面で入力した文字列をモデルに変換します。DBへの接続や問い合わせは不要です。
func buildQuery(draft Draft) (q.Select, error) {
	query := q.Select{From: q.Table{Name: strings.TrimSpace(draft.Table)}}
	for _, input := range strings.Split(draft.Columns, ",") {
		name := strings.TrimSpace(input)
		if name == "" {
			return q.Select{}, fmt.Errorf("取得列をカンマ区切りで入力してください")
		}
		expression := q.Col(name)
		if name == "*" {
			expression = q.Star()
		}
		query.Columns = append(query.Columns, q.Projection{Expr: expression})
	}
	if input := strings.TrimSpace(draft.Limit); input != "" {
		limit, err := strconv.ParseInt(input, 10, 64)
		if err != nil || limit < 0 {
			return q.Select{}, fmt.Errorf("件数は0以上の整数、または空欄にしてください")
		}
		query.Limit = q.Ptr(limit)
	}
	var filters []q.Expr
	if draft.ActiveOnly {
		filters = append(filters, q.Eq(q.Col("active"), q.Bool(true)))
	}
	if draft.NotDeleted {
		filters = append(filters, q.IsNull(q.Col("deleted_at")))
	}
	switch len(filters) {
	case 1:
		query.Where = q.Ptr(filters[0])
	case 2:
		query.Where = q.Ptr(q.And(filters...))
	}
	if err := q.Validate(query); err != nil {
		return q.Select{}, fmt.Errorf("入力を確認してください: %w", err)
	}
	return query, nil
}

// 学習用: 同じモデルからSQLとGoを生成します。画面部品やDBには依存しません。
func generatePreview(draft Draft) (Preview, error) {
	query, err := buildQuery(draft)
	if err != nil {
		return Preview{}, err
	}
	statement, err := sqlite.Compile(query)
	if err != nil {
		return Preview{}, err
	}
	source, err := gogen.Generate(query, "queries")
	if err != nil {
		return Preview{}, err
	}
	return Preview{SQL: statement.SQL, Args: statement.Args, Go: string(source)}, nil
}
