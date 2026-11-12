package execution

import "time"

// Column は結果の列名と、ドライバから取得できた型名です。
// DatabaseTypeName が空の場合は型名が不明です。
type Column struct {
	Name             string
	DatabaseTypeName string
}

// Result はSELECTの取得結果です。行の各値はColumnsと同じ順で並びます。
type Result struct {
	Columns   []Column
	Rows      [][]any
	Truncated bool
}

// Options は実行の行数上限と制限時間です。ゼロ値は既定値を選びます。
type Options struct {
	MaxRows int
	Timeout time.Duration
}

const (
	defaultMaxRows = 1000
	defaultTimeout = 5 * time.Second
)
