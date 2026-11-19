package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPreviewModelAndParameters(t *testing.T) {
	draft := sampleDraft()
	got, err := generatePreview(draft)
	want := `SELECT "id", "name" FROM "users" WHERE (("active" = ?) AND ("deleted_at" IS NULL)) LIMIT ?`
	if err != nil || got.SQL != want || !reflect.DeepEqual(got.Args, []any{true, int64(20)}) || !strings.Contains(got.Go, "q.Bool(true)") {
		t.Fatalf("preview = %#v, %v", got, err)
	}
	draft.Table = "accounts\"quoted"
	draft.Columns = "*"
	draft.ActiveOnly, draft.NotDeleted = false, false
	draft.Limit = "0"
	got, err = generatePreview(draft)
	if err != nil || got.SQL != `SELECT * FROM "accounts""quoted" LIMIT ?` || !reflect.DeepEqual(got.Args, []any{int64(0)}) {
		t.Fatalf("quoted table and zero limit = %#v, %v", got, err)
	}
	draft.Limit = ""
	got, err = generatePreview(draft)
	if err != nil || strings.Contains(got.SQL, "LIMIT") || len(got.Args) != 0 {
		t.Fatalf("no limit = %#v, %v", got, err)
	}
}

func TestPreviewRejectsInvalidInput(t *testing.T) {
	for _, draft := range []Draft{
		{Table: "users", Columns: ""},
		{Table: "", Columns: "id"},
		{Table: "users", Columns: "id,"},
		{Table: "users", Columns: "id", Limit: "-1"},
		{Table: "users", Columns: "id", Limit: "1.5"},
		{Table: "users", Columns: "id", Limit: "9223372036854775808"},
	} {
		if got, err := generatePreview(draft); err == nil || !reflect.DeepEqual(got, Preview{}) {
			t.Fatalf("invalid draft = %#v, %v", got, err)
		}
	}
}

func TestViewGenerateToggleAndReset(t *testing.T) {
	app := newApp()
	tester := ui.NewTester(app.view, 1000, 1000)
	if !tester.HasText(app.preview.SQL) {
		t.Fatalf("initial SQL is missing: %q", tester.Texts())
	}
	if err := tester.Click("active = true"); err != nil {
		t.Fatal(err)
	}
	if !tester.HasText("入力が変更されています。以下は前回の生成結果です。") {
		t.Fatal("stale preview is not marked")
	}
	if err := tester.Click("SQL / Goを生成"); err != nil {
		t.Fatal(err)
	}
	if app.draft.ActiveOnly || strings.Contains(app.preview.SQL, "active") || !tester.HasText(app.preview.SQL) {
		t.Fatalf("toggle did not change SQL: %#v", app.preview)
	}
	if err := tester.Click("サンプルに戻す"); err != nil {
		t.Fatal(err)
	}
	if app.draft != sampleDraft() || !strings.Contains(app.preview.SQL, "active") {
		t.Fatal("sample was not restored")
	}
}

func TestViewTypingErrorAndRecovery(t *testing.T) {
	app := newApp()
	tester := ui.NewTester(app.view, 1000, 1000)
	if err := tester.Click("テーブル名"); err != nil {
		t.Fatal(err)
	}
	tester.Key(ui.Cmd, ui.KeyA)
	tester.Type("orders")
	if err := tester.Click("SQL / Goを生成"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(app.preview.SQL, `FROM "orders"`) {
		t.Fatalf("typed table did not reach the model: %q", app.preview.SQL)
	}
	if err := tester.Click("最大件数"); err != nil {
		t.Fatal(err)
	}
	tester.Key(ui.Cmd, ui.KeyA)
	tester.Type("-1")
	if err := tester.Click("SQL / Goを生成"); err != nil {
		t.Fatal(err)
	}
	if app.message == "" || app.preview.SQL != "" || !tester.HasText(app.message) {
		t.Fatalf("error or cleared preview missing: %#v", app)
	}
	if err := tester.Click("サンプルに戻す"); err != nil {
		t.Fatal(err)
	}
	if app.message != "" || app.preview.SQL == "" || !tester.HasText(app.preview.SQL) {
		t.Fatal("preview did not recover")
	}
}
