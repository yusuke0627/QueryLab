package schema

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Read はSQLiteのmainスキーマを調べます。呼び出し元のDBは閉じません。
func Read(ctx context.Context, database *sql.DB) (Snapshot, error) {
	if ctx == nil || database == nil {
		return Snapshot{}, errors.New("read schema: context and database are required")
	}

	// 学習用: 一つのトランザクションで、複数の問い合わせが同じ時点の定義を見ます。
	// ReadOnly指定の強制はドライバに依存します。この処理ではSELECTだけを発行します。
	tx, err := database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Snapshot{}, fmt.Errorf("begin schema read: %w", err)
	}
	defer tx.Rollback()

	tables, err := readRows(ctx, tx, `
		SELECT entry.name, listing.type
		FROM main.sqlite_schema AS entry
		JOIN pragma_table_list AS listing ON listing.schema = 'main' AND listing.name = entry.name
		WHERE entry.type IN ('table', 'view')
		  AND listing.type IN ('table', 'view', 'virtual')
		  AND substr(entry.name, 1, 7) <> 'sqlite_'
		ORDER BY entry.name
	`, func(rows *sql.Rows) (Table, error) {
		var table Table
		err := rows.Scan(&table.Name, &table.Kind)
		return table, err
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("list schema objects: %w", err)
	}

	// 学習用: 一覧のRowsを閉じてから詳細へ進みます。途中で失敗したら部分結果は返しません。
	for index := range tables {
		if err := readTable(ctx, tx, &tables[index]); err != nil {
			return Snapshot{}, fmt.Errorf("read schema object %q: %w", tables[index].Name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Snapshot{}, fmt.Errorf("finish schema read: %w", errors.Join(err, ctx.Err()))
	}
	return Snapshot{Tables: tables}, nil
}

func readTable(ctx context.Context, tx *sql.Tx, table *Table) error {
	var err error
	// 学習用: 名前はSQLへ連結せず、? に渡します。mainの同名オブジェクトだけを参照します。
	table.Columns, err = readRows(ctx, tx, `
		SELECT name, type, "notnull", dflt_value, pk, hidden
		FROM pragma_table_xinfo(?, 'main') ORDER BY cid
	`, func(rows *sql.Rows) (Column, error) {
		var column Column
		err := rows.Scan(&column.Name, &column.DeclaredType, &column.NotNull,
			&column.DefaultSQL, &column.PrimaryKeyOrdinal, &column.HiddenCode)
		return column, err
	}, table.Name)
	if err != nil {
		return fmt.Errorf("list columns: %w", err)
	}
	if table.Kind == "view" {
		return nil
	}
	table.ForeignKeys, err = readForeignKeys(ctx, tx, table.Name)
	if err != nil {
		return fmt.Errorf("list foreign keys: %w", err)
	}
	table.Indexes, err = readIndexes(ctx, tx, table.Name)
	if err != nil {
		return fmt.Errorf("list indexes: %w", err)
	}
	return nil
}

type foreignKeyRow struct {
	key    ForeignKey
	column ForeignKeyColumn
}

func readForeignKeys(ctx context.Context, tx *sql.Tx, name string) ([]ForeignKey, error) {
	entries, err := readRows(ctx, tx, `
		SELECT id, "table", on_update, on_delete, "from", "to"
		FROM pragma_foreign_key_list(?, 'main') ORDER BY id, seq
	`, func(rows *sql.Rows) (foreignKeyRow, error) {
		var entry foreignKeyRow
		err := rows.Scan(&entry.key.ID, &entry.key.ReferencedTable, &entry.key.OnUpdate,
			&entry.key.OnDelete, &entry.column.From, &entry.column.To)
		return entry, err
	}, name)
	if err != nil {
		return nil, err
	}
	keys := make([]ForeignKey, 0)
	// 学習用: 同じIDの行は一つの外部キーです。複合キーは列の組を順にまとめます。
	for _, entry := range entries {
		last := len(keys) - 1
		if last < 0 || keys[last].ID != entry.key.ID {
			keys = append(keys, entry.key)
			last++
		}
		keys[last].Columns = append(keys[last].Columns, entry.column)
	}
	return keys, nil
}

func readIndexes(ctx context.Context, tx *sql.Tx, name string) ([]Index, error) {
	indexes, err := readRows(ctx, tx, `
		SELECT name, "unique", origin, partial
		FROM pragma_index_list(?, 'main') ORDER BY name
	`, func(rows *sql.Rows) (Index, error) {
		var index Index
		err := rows.Scan(&index.Name, &index.Unique, &index.Origin, &index.Partial)
		return index, err
	}, name)
	if err != nil {
		return nil, err
	}
	for position := range indexes {
		index := &indexes[position]
		index.Terms, err = readRows(ctx, tx, `
			SELECT cid, name, "desc", coll
			FROM pragma_index_xinfo(?, 'main') WHERE "key" = 1 ORDER BY seqno
		`, func(rows *sql.Rows) (IndexTerm, error) {
			var term IndexTerm
			err := rows.Scan(&term.ColumnID, &term.ColumnName, &term.Descending, &term.Collation)
			return term, err
		}, index.Name)
		if err != nil {
			return nil, fmt.Errorf("read index %q: %w", index.Name, err)
		}
	}
	return indexes, nil
}

// 学習用: Tは1行を保存する型です。scanだけを差し替え、後片付けは共通にします。
func readRows[T any](ctx context.Context, tx *sql.Tx, query string, scan func(*sql.Rows) (T, error), args ...any) ([]T, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]T, 0)
	for rows.Next() {
		value, err := scan(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return values, nil
}
