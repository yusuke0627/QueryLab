package execution

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	q "github.com/yusuke0627/QueryLab"
	"github.com/yusuke0627/QueryLab/dialect/sqlite"
)

// Execute はSELECTを実行して結果を返します。渡されたDB自体は閉じません。
func Execute(ctx context.Context, database *sql.DB, query q.Select, options Options) (Result, error) {
	if ctx == nil || database == nil {
		return Result{}, errors.New("execute: context and database are required")
	}
	if options.MaxRows < 0 || options.Timeout < 0 {
		return Result{}, errors.New("execute: row limit and timeout must not be negative")
	}
	if options.MaxRows == 0 {
		options.MaxRows = defaultMaxRows
	}
	if options.Timeout == 0 {
		options.Timeout = defaultTimeout
	}

	// 学習用: SQLとArgsは別々に受け取ります。コンパイルに失敗したらDBへは進みません。
	statement, err := sqlite.Compile(query)
	if err != nil {
		return Result{}, fmt.Errorf("compile select: %w", err)
	}

	// 学習用: deferはこの関数が戻るときに動きます。正常終了でもタイマーを片付けます。
	queryContext, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	// 学習用: Args...でスライスを展開し、SQLの?に対応する値を順番に渡します。
	rows, err := database.QueryContext(queryContext, statement.SQL, statement.Args...)
	if err != nil {
		return Result{}, executionError(queryContext, err)
	}
	defer rows.Close()

	result, err := collectRows(rows, options.MaxRows)
	if err != nil {
		return Result{}, executionError(queryContext, err)
	}
	// 学習用: deferは早期returnに備え、成功経路ではCloseの失敗も確認します。
	if err := rows.Close(); err != nil {
		return Result{}, executionError(queryContext, fmt.Errorf("close rows: %w", err))
	}
	if err := queryContext.Err(); err != nil {
		return Result{}, executionError(queryContext, err)
	}
	return result, nil
}

func executionError(ctx context.Context, err error) error {
	if contextErr := ctx.Err(); contextErr != nil && !errors.Is(err, contextErr) {
		err = errors.Join(err, contextErr)
	}
	return fmt.Errorf("execute select: %w", err)
}

func collectRows(rows *sql.Rows, maxRows int) (Result, error) {
	types, err := rows.ColumnTypes()
	if err != nil {
		return Result{}, fmt.Errorf("read column types: %w", err)
	}
	result := Result{Columns: make([]Column, len(types)), Rows: make([][]any, 0)}
	for index, column := range types {
		result.Columns[index] = Column{Name: column.Name(), DatabaseTypeName: column.DatabaseTypeName()}
	}
	destinations := make([]any, len(types))
	for rows.Next() {
		// 学習用: もう1行が存在すると分かったときだけ、打ち切りと判断します。
		if len(result.Rows) == maxRows {
			result.Truncated = true
			break
		}
		row := make([]any, len(types))
		for index := range row {
			// 学習用: &で書き込み先を渡します。値の型が列ごとに違うためanyを使います。
			destinations[index] = &row[index]
		}
		// 学習用: *anyへのScanでは、NULLはnilになり、バイト列はdatabase/sqlがコピーします。
		if err := rows.Scan(destinations...); err != nil {
			return Result{}, fmt.Errorf("scan row: %w", err)
		}
		result.Rows = append(result.Rows, row)
	}
	// 学習用: Nextがfalseになるのは読み終わった場合だけでなく、途中で失敗した場合もあります。
	if err := rows.Err(); err != nil {
		return Result{}, fmt.Errorf("read rows: %w", err)
	}
	return result, nil
}
