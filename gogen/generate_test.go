package gogen_test

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	q "github.com/yusuke0627/QueryLab"
	"github.com/yusuke0627/QueryLab/gogen"
)

func TestGenerateRejectsInvalidInput(t *testing.T) {
	query := q.Select{From: q.Table{Name: "users"}, Columns: []q.Projection{{Expr: q.Star()}}}
	for _, name := range []string{"", "package", "_", "bad-name", "queries; func init() {}"} {
		if _, err := gogen.Generate(query, name); err == nil {
			t.Errorf("accepted package name %q", name)
		}
	}
	if _, err := gogen.Generate(q.Select{}, "queries"); err == nil {
		t.Fatal("accepted invalid model")
	}
}

func TestGeneratedGoCompilesAndPreservesModel(t *testing.T) {
	query := q.Select{
		From: q.Table{Name: "users", Alias: "u"},
		Columns: []q.Projection{
			{Expr: q.Star()},
			{Expr: q.Ref("u", "id"), Alias: "user_id"},
			{Expr: q.Col("name")},
			{Expr: q.Text("日本語\n\"quoted\"\x00")},
			{Expr: q.Int(math.MinInt64)},
			{Expr: q.Int(math.MaxInt64)},
			{Expr: q.Real(1.25e-40)},
			{Expr: q.Real(1e100)},
			{Expr: q.Bool(false)},
			{Expr: q.Null()},
		},
		Where: q.Ptr(q.And(
			q.Compare(q.Greater, q.Ref("u", "age"), q.Int(20)),
			q.Or(q.IsNull(q.Col("deleted_at")), q.Not(q.Eq(q.Col("active"), q.Bool(true)))),
			q.IsNotNull(q.Col("name")),
		)),
		OrderBy: []q.Order{{Expr: q.Col("name"), Direction: q.Ascending}, {Expr: q.Col("id")}},
		Limit:   q.Ptr(int64(0)), Offset: q.Ptr(int64(2)), Distinct: true,
	}
	source, err := gogen.Generate(query, "queries")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "query.go", source, parser.AllErrors); err != nil {
		t.Fatal(err)
	}
	again, err := gogen.Generate(query, "queries")
	if err != nil || string(source) != string(again) {
		t.Fatalf("nondeterministic generation: %v", err)
	}
	encoded, err := json.Marshal(query)
	if err != nil {
		t.Fatal(err)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	moduleDirectory := filepath.Dir(workingDirectory)
	directory := t.TempDir()
	module := fmt.Sprintf("module generated.test/queries\n\ngo 1.23.0\n\nrequire github.com/yusuke0627/QueryLab v0.0.0\nreplace github.com/yusuke0627/QueryLab => %q\n", moduleDirectory)
	testSource := fmt.Sprintf(`package queries

import (
    "encoding/json"
    "reflect"
    "testing"
    q "github.com/yusuke0627/QueryLab"
    "github.com/yusuke0627/QueryLab/dialect/sqlite"
)

func TestRestoredModel(t *testing.T) {
    var expected q.Select
    if err := json.Unmarshal([]byte(%q), &expected); err != nil { t.Fatal(err) }
    if !reflect.DeepEqual(Query, expected) { t.Fatalf("generated model differs: %%#v", Query) }
    got, err := sqlite.Compile(Query)
    if err != nil { t.Fatal(err) }
    want, err := sqlite.Compile(expected)
    if err != nil { t.Fatal(err) }
    if !reflect.DeepEqual(got, want) { t.Fatal("generated query changed SQL or arguments") }
}
`, string(encoded))
	for name, content := range map[string][]byte{"go.mod": []byte(module), "query.go": source, "query_test.go": []byte(testSource)} {
		if err := os.WriteFile(filepath.Join(directory, name), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = directory
	command.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated Go failed to compile or roundtrip: %v\n%s\n%s", err, output, source)
	}
}

func TestGeneratedSourceUsesQuotedStrings(t *testing.T) {
	query := q.Select{From: q.Table{Name: "users"}, Columns: []q.Projection{{Expr: q.Text("`\n\"; panic(1) //")}}}
	source, err := gogen.Generate(query, "queries")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), `q.Text("`+"`"+`\n\"; panic(1) //")`) {
		t.Fatalf("text was not emitted as a quoted Go string: %s", source)
	}
}
