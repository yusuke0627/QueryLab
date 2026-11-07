# QueryLab

Goの宣言的SQLiteクエリビルダと、MyGoによるビジュアルクエリエディタ。
共通Query Modelを中心に、GUI・Goコード・SQL・SQLite実行結果をつなぎます。

管理リポジトリ: [yusuke0627/QueryLab](https://github.com/yusuke0627/QueryLab)。
Go module pathは`github.com/yusuke0627/QueryLab`です。

## このプロジェクトの目的

Go（Golang）、MyGo、クエリビルダのアルゴリズムを学ぶことを主目的とする開発プロジェクトです。
動く機能とともに、仕組み・設計判断・検証結果を自分の言葉で説明できる状態を目指します。
各実装段階の成果物には、レビュアーがコードと根拠をたどれる資料を含めます。

- [学習計画](docs/learning-plan.md): 各段階で学ぶ内容と理解を確認する課題。
- [レビュアー向けガイド](docs/review-guide.md): 読む順序、確認方法、レビュー観点。
- [M1 SELECTコアのレビュー資料](docs/reviews/m1-select-core.md): 実装済み機能の設計・アルゴリズム・テスト・制約。

## 現在の実装

- JSONに保存できる型付きQuery Modelと宣言的なGo API。
- SELECT / DISTINCT / FROM / 列・テーブルの別名。
- 比較（`=`, `<>`, `>`, `>=`, `<`, `<=`, `LIKE`）、AND / OR / NOT、NULL判定。
- ORDER BY / LIMIT / OFFSET。OFFSET単独にも対応。
- SQLite向けSQLと引数の生成、整形済みGoソースの生成。
- モデル検証、SQL生成、JSON往復、生成したGoのコンパイル・往復テスト。
- `sqlite3`があれば実際のSQLiteを使うSQL動作テスト。

SQLite実行API、MyGo GUI、JOIN、スキーマ取得、リレーション、SQL/Goの取り込みは今後の実装です。
今回の完了範囲と各フェーズの受け入れ条件は[実装計画](docs/implementation-plan.md)に記載しています。

## はじめる

Go 1.23以降を使用します。コアは標準ライブラリだけで動作します。

```sh
go test ./...
go run ./examples/select
```

`sqlite3` CLIはSQL動作テスト用の任意の依存です。未導入の場合、そのテストだけスキップします。
将来のSQLite実行APIはGoの`database/sql`で実装し、CLIには依存させません。

## 宣言的なクエリ

```go
import (
    q "github.com/yusuke0627/QueryLab"
    "github.com/yusuke0627/QueryLab/dialect/sqlite"
)

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
    Limit: q.Ptr(int64(20)),
}

statement, err := sqlite.Compile(query)
```

生成されるSQL:

```sql
SELECT "u"."id", "u"."name" AS "user_name" FROM "users" AS "u" WHERE (("u"."active" = ?) AND ("u"."deleted_at" IS NULL)) ORDER BY "u"."id" DESC LIMIT ?
```

`statement.Args`は`[]any{true, int64(20)}`です。値はSQLに埋め込みません。
`Col("a.b")`は名前が`a.b`の単一の列を表します。修飾列には`Ref("a", "b")`を使います。
`Eq(column, Null())`は検証エラーです。NULL判定には`IsNull` / `IsNotNull`を使います。
LIMIT未指定は`nil`、0行に制限する場合は`q.Ptr(int64(0))`です。

## Go生成

```go
source, err := gogen.Generate(query, "queries")
```

`github.com/yusuke0627/QueryLab/gogen`をimportして使います。
指定パッケージの`var Query = q.Select{...}`を持つGoファイルを生成します。
現時点の方向はModel → SQL / Goです。SQL → Model、Go → Modelは限定的双方向編集フェーズで追加します。

## 構成

```text
model.go, expr.go, validate.go   共通Query Model、宣言的API、構造検証
dialect/sqlite/                 SQLite SQLと引数の生成
gogen/                          宣言的Goソース生成
examples/select/                実行できるSELECT例
docs/design.md                  設計と判断理由
docs/implementation-plan.md     段階的な実装計画
docs/learning-plan.md           学習目標と理解確認の課題
docs/review-guide.md            レビュアー向けの入口と共通確認事項
docs/reviews/                   実装段階ごとのレビュー資料
AGENTS.md                       Codex向け開発ルール
```

クエリの構造検証は実装済みですが、テーブルや列の存在確認はスキーマ取得フェーズで追加します。

## ロードマップ

1. SELECTコアとModel → SQL / Go（初期実装）。
2. SQLite実行、結果列・型・NULL、キャンセル、スキーマ取得。
3. 明示的JOIN、外部キーに基づくリレーション候補、結果のマッピング。
4. MyGo GUI、テーブル・列・条件・JOINの視覚編集、SQL/Go/結果の同期表示。
5. 対応構文を明示したSQL/Go → Modelの取り込みと限定的双方向編集。

MyGoの具体的な依存先・バージョン・接続方式はGUIフェーズの調査で確定します。
参照会話から提案本文を取得できなかったため、本リポジトリは引き継ぎ依頼に列挙された合意事項を基準にしています。
