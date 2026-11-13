package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/yusuke0627/QueryLab/schema"
	sqlitedb "github.com/yusuke0627/QueryLab/storage/sqlite"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: go run ./examples/schema <database-file>")
	}
	if err := run(os.Args[1]); err != nil {
		log.Fatal(err)
	}
}

func run(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := sqlitedb.OpenReadOnly(ctx, path)
	if err != nil {
		return err
	}
	defer db.Close()

	// 学習用: 実データを検索するExecuteとは別に、DBの定義をReadで取得します。
	snapshot, err := schema.Read(ctx, db)
	if err != nil {
		return err
	}
	output := json.NewEncoder(os.Stdout)
	output.SetIndent("", "  ")
	return output.Encode(snapshot)
}
