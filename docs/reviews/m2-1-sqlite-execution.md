# M2-1 SQLite実行のレビュー

## 状態

SELECTの読み取り専用実行、行の収集、件数上限、キャンセルを実装済みです。
[タスク設計とドライバ選定](../tasks/m2-1-sqlite-execution.md)も参照してください。
スキーマ取得、JOIN、MyGo GUI、ソース取り込みは対象外です。

## 読む順序と学習目標

1. [SQL生成の例](../../examples/select/main.go)と[DB実行の例](../../examples/execute/main.go)を比較する。
2. [OpenReadOnly](../../storage/sqlite/open.go)で引数・戻り値と、DBを閉じる責務を読む。
3. [ExecuteとcollectRows](../../execution/execute.go)で、正常・異常時の片付けと行の読み方を追う。
4. [結果モデル](../../execution/result.go)と下記のテストで、列位置、上限、エラーを確認する。

重点は、普通の関数、複数の戻り値、ポインタ、スライス、`defer`、`context`です。
初期版で独自interfaceやRunner型を導入しない理由も説明できることを目指します。

## 設計で確認すること

| 観点 | 実装 | レビューする理由 |
| --- | --- | --- |
| 境界 | 実行層はSelectとsql.DBを受け取り、内部でCompileする | 任意SQLを公開APIの入力にせず、既存の検証を使う |
| 所有権 | DBは呼び出し元、Rowsは実行関数が閉じる | 複数回の実行と失敗時の解放を両立する |
| 結果 | 列情報と位置対応する行配列 | 同名列とNULLを失わない |
| 型 | 値はドライバの型を保持、バイト列はコピー | 宣言型と実際の値を混同せず、返した結果を保持できる |
| 上限 | 既定1000行、追加の1行確認で打ち切りを識別 | ちょうど上限件数の場合を誤って打ち切りとしない |
| 期限 | 既定5秒、呼び出し元の中止を伝える | 実行の終了と資源解放を学ぶ |
| エラー | 部分結果を返さず、原因をラップする | 取得失敗を成功したデータとして表示しない |
| ドライバ | go-sqlite3 v1.14.53を固定し、Go 1.23を維持 | moderncの現行版に必要なGo更新と、採用版のCGO要件を比較 |

## 確認したい制約

件数上限はDBの検索コストや、巨大な単一セルのメモリを制限しません。
キャンセルが行評価中に伝わることは確認しましたが、中断時間や他のOSでの動作は保証しません。
実行例と全体テストにはCGOとCコンパイラが必要です。クエリモデル・生成器はCGOに依存しません。
アダプターの処理は列数C・取得行数R・コピーしたバイト数Bに対してO((R+1)×C+B)を目安とします。
SQLite自身の検索コストを含まない分析であり、性能の実測値ではありません。
MyGo・JOIN・スキーマ取得・ソース取り込みの順序は維持しています。

## 学習者への質問

- OpenReadOnlyとExecuteの引数・戻り値を、一つずつ自分の言葉で説明できるか。
- `result, err := Execute(...)`の後、なぜ先にerrを確認するのか。
- 同じDBで2回検索する場合、なぜExecuteでDBを閉じないのか。
- `Next`がfalseになっただけでは、なぜ読み取り成功と判断できないのか。
- 1000行返しただけでは、なぜTruncatedをtrueにできないのか。
- `Bool(true)`を条件に使ったとき、返ってくる列値も必ずboolになると言えるか。
- contextを渡すことと、ドライバが実行中に中止できることはどう違うか。

## テストと再現手順

| テスト | 確認内容 |
| --- | --- |
| [実SQLiteの接続テスト](../../storage/sqlite/open_test.go) | 特殊文字を含むパス、書き込み拒否、存在しないDBを作らないこと、事前キャンセル |
| [実SQLiteの実行テスト](../../execution/execute_test.go) | bool・文字列のバインド、同名列・NULL・BLOB、上限の境界とLIMIT=0、閉じたDB、期限切れと読み取り中キャンセル、再実行 |
| [故障を模擬するテスト](../../execution/rows_failure_test.go) | Scan・読み取り・Closeの失敗時に部分結果を返さずRowsを閉じること |

リポジトリのルートで`CGO_ENABLED=1 go test ./execution ./storage/sqlite -count=1`を実行できます。
[READMEのデモ](../../README.md#sqliteで実行する)では、SQLの条件に一致する行だけが取得されます。
取得した列は`id`と`user_name`、行は`[1 Alice]`です。

2026-11-12、macOS arm64 / Apple clang 21での確認結果:

- Go 1.23.12: `CGO_ENABLED=1 go test ./... -count=1` 成功。
- Go 1.23.12: `CGO_ENABLED=0 go test ./ ./dialect/sqlite ./gogen -count=1` 成功。
- Go 1.25.7: `CGO_ENABLED=1 go vet ./...` と `CGO_ENABLED=1 go test -race ./... -count=1` 成功。競合チェックのリンク時に警告が出たが、テストは成功。
- READMEの一時DBで実行例を再現し、`[1 Alice]`を確認。
