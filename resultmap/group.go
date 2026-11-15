package resultmap

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"

	"github.com/yusuke0627/QueryLab/execution"
)

type Cardinality int

const (
	OneToOne Cardinality = iota + 1
	OneToMany
)

// Selection は結果の列番号（先頭は0）で、キーと返す値を指定します。
type Selection struct {
	Key    []int
	Values []int
}

type Spec struct {
	Parent      Selection
	Child       Selection
	Cardinality Cardinality
}

// Group は親の値と子の値を持ちます。OneToOneでも子は0件または1件のスライスです。
type Group struct {
	Parent   []any
	Children [][]any
}

type groupState struct {
	index    int
	children map[string]int
}

// Nest は完全な平坦結果を、指定したキーで親と子にまとめます。入力は変更しません。
func Nest(result execution.Result, spec Spec) ([]Group, error) {
	if result.Truncated {
		return nil, fmt.Errorf("cannot nest truncated results")
	}
	if spec.Cardinality != OneToOne && spec.Cardinality != OneToMany {
		return nil, fmt.Errorf("unsupported cardinality %d", spec.Cardinality)
	}
	if err := validateSelection("parent", spec.Parent, len(result.Columns)); err != nil {
		return nil, err
	}
	if err := validateSelection("child", spec.Child, len(result.Columns)); err != nil {
		return nil, err
	}

	groups := make([]Group, 0)
	parents := make(map[string]*groupState)
	// 学習用: mapは「このキーを前に見たか」の検索に使い、スライスは最初に出た順を保ちます。
	for rowIndex, row := range result.Rows {
		if len(row) != len(result.Columns) {
			return nil, fmt.Errorf("row %d: got %d values for %d columns", rowIndex, len(row), len(result.Columns))
		}
		parentKey, missingParent, err := rowKey(row, spec.Parent.Key)
		if err != nil || missingParent {
			if err != nil {
				return nil, fmt.Errorf("row %d parent key: %w", rowIndex, err)
			}
			return nil, fmt.Errorf("row %d: parent key is NULL", rowIndex)
		}
		state, exists := parents[parentKey]
		if !exists {
			state = &groupState{index: len(groups), children: make(map[string]int)}
			parents[parentKey] = state
			groups = append(groups, Group{Parent: project(row, spec.Parent.Values), Children: make([][]any, 0)})
		}
		group := &groups[state.index]
		if exists && !matches(group.Parent, row, spec.Parent.Values) {
			return nil, fmt.Errorf("row %d: conflicting parent values for the same key", rowIndex)
		}

		childKey, missingChild, err := rowKey(row, spec.Child.Key)
		if err != nil {
			return nil, fmt.Errorf("row %d child key: %w", rowIndex, err)
		}
		// 学習用: LEFT JOINで子が見つからないと、子キーはすべてNULLになります。
		if missingChild {
			continue
		}
		if index, exists := state.children[childKey]; exists {
			if !matches(group.Children[index], row, spec.Child.Values) {
				return nil, fmt.Errorf("row %d: conflicting child values for the same key", rowIndex)
			}
			continue
		}
		if spec.Cardinality == OneToOne && len(group.Children) != 0 {
			return nil, fmt.Errorf("row %d: more than one child for a one-to-one parent", rowIndex)
		}
		state.children[childKey] = len(group.Children)
		group.Children = append(group.Children, project(row, spec.Child.Values))
	}
	return groups, nil
}

func validateSelection(role string, selection Selection, width int) error {
	for _, field := range []struct {
		name    string
		columns []int
	}{{"key", selection.Key}, {"values", selection.Values}} {
		if len(field.columns) == 0 {
			return fmt.Errorf("%s %s must specify at least one column", role, field.name)
		}
		seen := make(map[int]bool)
		for _, index := range field.columns {
			if index < 0 || index >= width || seen[index] {
				return fmt.Errorf("%s %s: invalid or repeated column index %d", role, field.name, index)
			}
			seen[index] = true
		}
	}
	return nil
}

func rowKey(row []any, columns []int) (string, bool, error) {
	var key []byte
	nulls := 0
	for _, index := range columns {
		switch value := row[index].(type) {
		case nil:
			nulls++
		case int64:
			key = strconv.AppendInt(append(key, 'i'), value, 10)
			key = append(key, ';')
		case string:
			// 学習用: 型と文字列のバイト数を付けると、値に区切り文字があっても混同しません。
			key = strconv.AppendInt(append(key, 's'), int64(len(value)), 10)
			key = append(append(key, ':'), value...)
		default:
			return "", false, fmt.Errorf("column %d: expected int64 or string, got %T", index, value)
		}
	}
	if nulls > 0 && nulls < len(columns) {
		return "", false, fmt.Errorf("partially NULL key")
	}
	return string(key), nulls == len(columns), nil
}

func project(row []any, columns []int) []any {
	values := make([]any, len(columns))
	for position, index := range columns {
		value := row[index]
		if data, ok := value.([]byte); ok {
			// 学習用: BLOBをコピーし、出力の変更が元の平坦な結果に伝わらないようにします。
			value = bytes.Clone(data)
		}
		values[position] = value
	}
	return values
}

func matches(values, row []any, columns []int) bool {
	for position, index := range columns {
		if !reflect.DeepEqual(values[position], row[index]) {
			return false
		}
	}
	return true
}
