package main

import (
	"fmt"
	"log"

	q "github.com/yusuke0627/QueryLab"
	"github.com/yusuke0627/QueryLab/dialect/sqlite"
	"github.com/yusuke0627/QueryLab/gogen"
)

func main() {
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
	statement, err := sqlite.Compile(query)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(statement.SQL)
	fmt.Printf("args: %#v\n\n", statement.Args)
	source, err := gogen.Generate(query, "queries")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(string(source))
}
