package main

import (
	"fmt"
	"log"

	q "github.com/yusuke0627/QueryLab"
	"github.com/yusuke0627/QueryLab/dialect/sqlite"
	"github.com/yusuke0627/QueryLab/gogen"
)

func main() {
	// 学習用: qはimportしたQueryLabパッケージの短い名前です。
	// q.Select{...}で検索内容を宣言します。この時点ではDBに接続しません。
	query := q.Select{
		From: q.Table{Name: "users", Alias: "u"},
		Columns: []q.Projection{
			{Expr: q.Ref("u", "id")},
			{Expr: q.Ref("u", "name"), Alias: "user_name"},
		},
		// 学習用: Andで2条件を結びます。Bool(true)は比較用の値のExprです。
		// 「activeがtrue」かつ「deleted_atがNULL」というWHEREを作ります。
		// PtrはExprを*Exprにし、Whereフィールドに渡せる形にします。
		Where: q.Ptr(q.And(
			q.Eq(q.Ref("u", "active"), q.Bool(true)),
			q.IsNull(q.Ref("u", "deleted_at")),
		)),
		OrderBy: []q.Order{{Expr: q.Ref("u", "id"), Direction: q.Descending}},
		// 学習用: int64(20)で整数の型を明示し、Ptrで*int64にします。
		// nilならLIMITなしですが、ここでは20行までと指定します。
		Limit: q.Ptr(int64(20)),
	}
	// 学習用: Compileでモデルを検証し、SQL文字列と別管理の引数に変換します。
	// 生成するだけなので、ここでもDBへの問い合わせは行いません。
	statement, err := sqlite.Compile(query)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(statement.SQL)
	fmt.Printf("args: %#v\n\n", statement.Args)
	// 学習用: SQL中の?には順番にtrueとint64(20)が対応します。
	// 同じモデルを、queriesパッケージのGoソースにも変換します。
	source, err := gogen.Generate(query, "queries")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(string(source))
}
