package schema_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/yusuke0627/QueryLab/schema"
	sqlitedb "github.com/yusuke0627/QueryLab/storage/sqlite"
)

func openFixture(t *testing.T, ddl string) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "schema.db")
	setup, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	_, setupErr := setup.Exec(ddl)
	if err := errors.Join(setupErr, setup.Close()); err != nil {
		t.Fatal(err)
	}
	db, err := sqlitedb.OpenReadOnly(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestReadMetadata(t *testing.T) {
	db := openFixture(t, `
		CREATE TABLE parent (first TEXT, second INTEGER, PRIMARY KEY(second, first)) WITHOUT ROWID;
		CREATE TABLE "child's ?" (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			first TEXT NOT NULL DEFAULT 'test',
			second INTEGER DEFAULT NULL,
			derived TEXT GENERATED ALWAYS AS (lower(first)) STORED,
			FOREIGN KEY(second, first) REFERENCES parent(second, first) ON DELETE CASCADE,
			FOREIGN KEY(first, second) REFERENCES parent ON UPDATE CASCADE
		);
		CREATE UNIQUE INDEX "child's index ?" ON "child's ?"(second DESC, lower(first)) WHERE second IS NOT NULL;
		CREATE VIEW child_view AS SELECT id FROM "child's ?";
		CREATE VIRTUAL TABLE search_docs USING fts4(content);
	`)
	// 学習用: TEMPに同名のテーブルがあっても、main側の定義を取得します。
	if _, err := db.Exec(`CREATE TEMP TABLE "child's ?" (temp_only TEXT)`); err != nil {
		t.Fatal(err)
	}
	snapshot, err := schema.Read(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	tables := snapshot.Tables
	if len(tables) != 4 || tables[0].Name != "child's ?" || tables[1].Name != "child_view" ||
		tables[2].Name != "parent" || tables[3].Name != "search_docs" || tables[3].Kind != "virtual" {
		t.Fatalf("unexpected objects: %#v", tables)
	}
	child := tables[0]
	if len(child.Columns) != 4 {
		t.Fatalf("unexpected columns: %#v", child.Columns)
	}
	id, first, second, derived := child.Columns[0], child.Columns[1], child.Columns[2], child.Columns[3]
	if id.Name != "id" || id.PrimaryKeyOrdinal != 1 || id.DefaultSQL != nil ||
		first.DeclaredType != "TEXT" || !first.NotNull || first.DefaultSQL == nil || *first.DefaultSQL != "'test'" ||
		second.DefaultSQL == nil || *second.DefaultSQL != "NULL" || derived.Name != "derived" || derived.HiddenCode != 3 {
		t.Fatalf("unexpected columns: %#v", child.Columns)
	}
	parent := tables[2]
	if parent.Columns[0].PrimaryKeyOrdinal != 2 || parent.Columns[1].PrimaryKeyOrdinal != 1 {
		t.Fatalf("unexpected primary-key order: %#v", parent.Columns)
	}
	view := tables[1]
	if view.Kind != "view" || len(view.Columns) != 1 || view.Columns[0].Name != "id" ||
		len(view.Indexes) != 0 || len(view.ForeignKeys) != 0 {
		t.Fatalf("unexpected view: %#v", view)
	}
	checkForeignKeys(t, child.ForeignKeys)
	if len(child.Indexes) != 1 {
		t.Fatalf("unexpected indexes: %#v", child.Indexes)
	}
	index := child.Indexes[0]
	if index.Name != "child's index ?" || !index.Unique || !index.Partial || index.Origin != "c" || len(index.Terms) != 2 {
		t.Fatalf("unexpected index metadata: %#v", index)
	}
	if term := index.Terms[0]; term.ColumnName == nil || *term.ColumnName != "second" || !term.Descending || term.Collation != "BINARY" {
		t.Fatalf("unexpected column term: %#v", term)
	}
	if term := index.Terms[1]; term.ColumnID != -2 || term.ColumnName != nil {
		t.Fatalf("unexpected expression term: %#v", term)
	}
	again, err := schema.Read(context.Background(), db)
	if err != nil || !reflect.DeepEqual(snapshot, again) {
		t.Fatalf("schema changed on repeated read: %#v, %v", again, err)
	}
}

func checkForeignKeys(t *testing.T, keys []schema.ForeignKey) {
	t.Helper()
	if len(keys) != 2 || keys[0].ID == keys[1].ID {
		t.Fatalf("unexpected keys: %#v", keys)
	}
	for _, key := range keys {
		if key.ReferencedTable != "parent" || len(key.Columns) != 2 {
			t.Fatalf("unexpected key: %#v", key)
		}
		first, second := key.Columns[0], key.Columns[1]
		switch {
		case key.OnDelete == "CASCADE":
			if first.From != "second" || second.From != "first" ||
				first.To == nil || *first.To != "second" || second.To == nil || *second.To != "first" {
				t.Fatalf("unexpected explicit column pairs: %#v", key)
			}
		case key.OnUpdate == "CASCADE":
			if first.From != "first" || second.From != "second" || first.To != nil || second.To != nil {
				t.Fatalf("unexpected implicit column pairs: %#v", key)
			}
		default:
			t.Fatalf("unexpected foreign-key actions: %#v", key)
		}
	}
}

func TestReadEmptyAndCanceled(t *testing.T) {
	db := openFixture(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := schema.Read(ctx, db); !errors.Is(err, context.Canceled) || result.Tables != nil {
		t.Fatalf("canceled read = %#v, %v", result, err)
	}
	result, err := schema.Read(context.Background(), db)
	if err != nil || result.Tables == nil || len(result.Tables) != 0 {
		t.Fatalf("empty schema = %#v, %v", result, err)
	}
}

func TestReadDiscardsPartialSnapshotAndReleasesConnection(t *testing.T) {
	db := openFixture(t, `CREATE TABLE available (id INTEGER); CREATE VIEW broken AS SELECT * FROM missing;`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if result, err := schema.Read(ctx, db); err == nil || result.Tables != nil {
		t.Fatalf("broken view = %#v, %v", result, err)
	}
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("connection unavailable after failed read: %v", err)
	}
}
