package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	q "github.com/yusuke0627/QueryLab"
	"github.com/yusuke0627/QueryLab/execution"
	sqlitedb "github.com/yusuke0627/QueryLab/storage/sqlite"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: go run ./examples/execute <database-file>")
	}
	if err := run(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}

func run(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	database, err := sqlitedb.OpenReadOnly(ctx, path)
	if err != nil {
		return err
	}
	// 学習用: DBを開いたrunが閉じます。途中で失敗してもdeferは実行されます。
	defer database.Close()

	query := q.Select{
		From: q.Table{Name: "users", Alias: "u"},
		Columns: []q.Projection{
			{Expr: q.Ref("u", "id")},
			{Expr: q.Ref("u", "name"), Alias: "user_name"},
		},
		Where: q.Ptr(q.And(
			q.Eq(q.Ref("u", "active"), q.Bool(true)),
			q.IsNull(q.Ref("u", "deleted_at")),
		)),
		OrderBy: []q.Order{{Expr: q.Ref("u", "id"), Direction: q.Descending}},
		Limit:   q.Ptr(int64(20)),
	}
	// 学習用: 2つの戻り値を受け取り、結果を使う前にエラーを確認します。
	result, err := execution.Execute(ctx, database, query, execution.Options{})
	if err != nil {
		return err
	}
	for _, column := range result.Columns {
		fmt.Printf("%s\t", column.Name)
	}
	fmt.Println()
	for _, row := range result.Rows {
		fmt.Println(row)
	}
	if result.Truncated {
		fmt.Println("result truncated")
	}
	return nil
}
