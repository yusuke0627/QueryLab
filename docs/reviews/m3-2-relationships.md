# M3-2 外部キーからJOIN候補を作る

## 目的と使い方

`orders.customer_id` が `customers.id` を参照しているなら、その列の組をJOIN候補として返します。
配送先と請求先のように外部キーが2本ある場合は2候補です。候補を取得しても、クエリは変わりません。

```go
candidates, err := relationship.Between(snapshot, "orders", "customers")
```

`snapshot` は `schema.Read` の結果です。検索結果の行は不要です。
`err` を先に確認し、候補の列の組（`Pairs`）を見て使うものを選びます。
選んだ候補を `selected` とすると、次の関数でJOINの部品を一つ作れます。

```go
join, err := selected.Join(q.LeftJoin,
    q.Table{Name: "orders", Alias: "o"},
    q.Table{Name: "customers", Alias: "c"},
)
```

成功したら、呼び出し側が `query.Joins` に追加します。左側の別名 `o` は、クエリですでに使っている別名と揃えます。

## 読み順と判断

1. [候補取得とJOIN変換](../../relationship/relationship.go)の `Between` → `resolvePairs` → `Join` の順で読みます。
2. [スキーマの型](../../schema/model.go)で、外部キーの参照先列が省略されると `To` が `nil` になることを確認します。
3. [テスト](../../relationship/relationship_test.go)で、実SQLiteから取得した定義とJOIN結果を比べます。

- 左右どちらが外部キーを持つかを別々に調べます。自己参照では「社員→上司」と「上司→部下」の両方を返します。
- 参照先列が省略されていたら、列の表示順ではなく、主キーの中での順番で対応づけます。複合キーの条件はすべて `AND` でつなぎます。
- SQLiteは外部キーの文字列比較に親列の照合順序を使います。生成する等号でも親列を先に置きます。ただしJOINは普通のSQLの等号で、型変換も含めた外部キー検査そのものではありません。[SQLiteの外部キー仕様](https://www.sqlite.org/foreignkeys.html#fk_basics)
- SQL文字列を直接作る案ではなく、既存の `q.Join` を返します。留保していたユーザーの選択をここで確定し、既存の検証・SQL生成・Go生成を再利用します。

探索はテーブル一覧から両端を探し、両端の外部キーだけを確認します。列名を線形探索するため、列の組がK個なら列照合は最大でおおむね `K ×（両テーブルの列数）` 回です。索引化はせず、読みやすさを優先しました。性能測定はしていません。

## 制約

直接の関係のみです。複数テーブルを経由する経路探索、候補の自動選択、外部キー制約の有効性や行データの整合性の検査、結果のネストは行いません。
選んだ二表をつなぐ外部キーが解決できない場合はエラーにし、候補を一部だけ返しません。

## 検証と確認問題

```sh
CGO_ENABLED=1 go test ./relationship -count=1
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go vet ./...
```

実SQLiteテストでは、二つの候補のうち選んだ複合キーでだけ結合し、全列一致・文字列の照合順序・LEFT JOINのNULLを確認します。
別テストで逆向きの列の組、自己参照の二方向、別名の重複、候補がない場合、不正なメタデータを確認します。
macOS arm64でGo 1.23.12の全体テストとGo 1.25.7の静的チェックが成功しました。

- 親の列が `first, second` の順でも、主キーが `PRIMARY KEY(second, first)` なら、参照先省略時の最初の組はどの列ですか？
- 社員から上司への候補と、上司から部下への候補では、何を入れ替えますか？
- 候補が一つしかなくても、なぜ `Between` はクエリにJOINを追加しないのでしょうか？
