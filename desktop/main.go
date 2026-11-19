package main

import (
	"fmt"
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type app struct {
	draft     Draft
	generated Draft
	preview   Preview
	message   string
}

func newApp() *app {
	app := &app{draft: sampleDraft()}
	app.generate()
	return app
}

func (app *app) generate() {
	preview, err := generatePreview(app.draft)
	app.generated = app.draft
	app.preview = preview
	app.message = ""
	if err != nil {
		app.message = err.Error()
	}
}

// 学習用: MyGoは入力などに応じてviewを再実行します。画面部品はこの呼び出しの外へ保存しません。
func (app *app) view(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(28).Gap(12).Children(func() {
			ui.Text(c, "QueryLab").FontSize(28).Bold()
			ui.Text(c, "M4-1 · クエリのプレビュー").TextColor(c.Theme().TextMuted)
			ui.Text(c, "DBには接続しません。入力からSQLとGoコードを生成します。")
			ui.TextInput(c, &app.draft.Table).Label("テーブル名").Placeholder("users").FillWidth()
			ui.TextInput(c, &app.draft.Columns).Label("取得列").Placeholder("id, name").FillWidth()
			ui.Text(c, "列はカンマ区切り。全列を指定する場合は * を入力します。").TextColor(c.Theme().TextMuted)
			ui.TextInput(c, &app.draft.Limit).Label("最大件数").Placeholder("空欄なら制限なし").Width(220)
			ui.Checkbox(c, &app.draft.ActiveOnly, "active = true")
			ui.Checkbox(c, &app.draft.NotDeleted, "deleted_at IS NULL")
			ui.Row(c).Gap(12).Children(func() {
				if ui.PrimaryButton(c, "SQL / Goを生成").Clicked() {
					app.generate()
				}
				if ui.Button(c, "サンプルに戻す").Clicked() {
					app.draft = sampleDraft()
					app.generate()
				}
			})
			if app.draft != app.generated {
				ui.Text(c, "入力が変更されています。以下は前回の生成結果です。").Bold()
			}
			if app.message != "" {
				ui.Text(c, app.message).Bold()
				return
			}
			ui.Text(c, "SQL").FontSize(18).Bold()
			ui.Text(c, app.preview.SQL).Selectable()
			ui.Textf(c, "引数: %#v", app.preview.Args).Selectable()
			ui.Text(c, "Goコード").FontSize(18).Bold()
			ui.Text(c, app.preview.Go).Selectable()
		})
	})
}

func main() {
	app := newApp()
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title: "QueryLab", Width: 1000, Height: 900, MinWidth: 640, MinHeight: 600,
			Content: ui.View(app.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(fmt.Errorf("QueryLab: %w", err))
	}
}
