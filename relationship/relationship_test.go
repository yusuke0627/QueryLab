package relationship_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	q "github.com/yusuke0627/QueryLab"
	"github.com/yusuke0627/QueryLab/execution"
	"github.com/yusuke0627/QueryLab/relationship"
	"github.com/yusuke0627/QueryLab/schema"
	sqlitedb "github.com/yusuke0627/QueryLab/storage/sqlite"
)

func TestBetweenAndExecuteSelectedCompositeJoin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.db")
	setup, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	_, setupErr := setup.Exec(`
		CREATE TABLE parent (first TEXT COLLATE NOCASE, second INTEGER, PRIMARY KEY(second, first));
		CREATE TABLE child (id INTEGER, code TEXT, number INTEGER, alternate INTEGER,
		    FOREIGN KEY(number, code) REFERENCES PARENT,
		    FOREIGN KEY(alternate, code) REFERENCES parent(second, first));
		INSERT INTO parent VALUES ('ABC', 1), ('OTHER', 2);
		INSERT INTO child VALUES (10, 'abc', 1, 2), (11, 'abc', 2, 1), (12, NULL, 1, 1);
	`)
	if err := errors.Join(setupErr, setup.Close()); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	db, err := sqlitedb.OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	snapshot, err := schema.Read(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := relationship.Between(snapshot, "CHILD", "parent")
	if err != nil || len(candidates) != 2 {
		t.Fatalf("candidates = %#v, %v", candidates, err)
	}
	var selected relationship.Candidate
	for _, candidate := range candidates {
		if reflect.DeepEqual(candidate.Pairs, []relationship.Pair{
			{LeftColumn: "number", RightColumn: "second"}, {LeftColumn: "code", RightColumn: "first"},
		}) {
			selected = candidate
		}
	}
	left, right := q.Table{Name: "child", Alias: "c"}, q.Table{Name: "parent", Alias: "p"}
	join, err := selected.Join(q.LeftJoin, left, right)
	if err != nil {
		t.Fatal(err)
	}
	result, err := execution.Execute(ctx, db, q.Select{
		From: left, Joins: []q.Join{join},
		Columns: []q.Projection{{Expr: q.Ref("c", "id")}, {Expr: q.Ref("p", "first")}},
		OrderBy: []q.Order{{Expr: q.Ref("c", "id")}},
	}, execution.Options{})
	if want := ([][]any{{int64(10), "ABC"}, {int64(11), nil}, {int64(12), nil}}); err != nil || !reflect.DeepEqual(result.Rows, want) {
		t.Fatalf("rows = %#v, %v; want %#v", result.Rows, err, want)
	}
	reversed, err := relationship.Between(snapshot, "parent", "child")
	if err != nil || len(reversed) != 2 {
		t.Fatalf("reverse candidates = %#v, %v", reversed, err)
	}
	for index, candidate := range reversed {
		original := candidates[index]
		if candidate.ForeignKeyOnLeft || candidate.ForeignKeyID != original.ForeignKeyID {
			t.Fatalf("unexpected reverse candidate: %#v", candidate)
		}
		for position, pair := range candidate.Pairs {
			if pair.LeftColumn != original.Pairs[position].RightColumn || pair.RightColumn != original.Pairs[position].LeftColumn {
				t.Fatalf("pair not reversed: %#v", pair)
			}
		}
	}
}

func TestSelfRelationshipRequiresAliasesAndPreservesDirection(t *testing.T) {
	target := "id"
	snapshot := schema.Snapshot{Tables: []schema.Table{
		{Name: "employees", Columns: []schema.Column{{Name: "id"}, {Name: "manager_id"}},
			ForeignKeys: []schema.ForeignKey{{ReferencedTable: "employees", Columns: []schema.ForeignKeyColumn{{From: "manager_id", To: &target}}}}},
		{Name: "unrelated"},
	}}
	candidates, err := relationship.Between(snapshot, "employees", "employees")
	if err != nil || len(candidates) != 2 {
		t.Fatalf("self candidates = %#v, %v", candidates, err)
	}
	if !candidates[0].ForeignKeyOnLeft || candidates[1].ForeignKeyOnLeft ||
		candidates[0].Pairs[0] != (relationship.Pair{LeftColumn: "manager_id", RightColumn: "id"}) ||
		candidates[1].Pairs[0] != (relationship.Pair{LeftColumn: "id", RightColumn: "manager_id"}) {
		t.Fatalf("unexpected directions: %#v", candidates)
	}
	employee, manager := q.Table{Name: "employees", Alias: "worker"}, q.Table{Name: "employees", Alias: "manager"}
	for _, test := range []struct {
		kind  q.JoinKind
		right q.Table
	}{
		{q.LeftJoin, employee},
		{q.InnerJoin, q.Table{Name: "unrelated"}},
		{"CROSS", manager},
	} {
		if _, err := candidates[0].Join(test.kind, employee, test.right); err == nil {
			t.Fatalf("accepted invalid join: %#v", test)
		}
	}
	join, err := candidates[0].Join(q.LeftJoin, employee, manager)
	if err != nil || !reflect.DeepEqual(join.On, q.Eq(q.Ref("manager", "id"), q.Ref("worker", "manager_id"))) {
		t.Fatalf("self join = %#v, %v", join, err)
	}
	if unrelated, err := relationship.Between(snapshot, "employees", "unrelated"); err != nil || len(unrelated) != 0 {
		t.Fatalf("unrelated = %#v, %v", unrelated, err)
	}
	if _, err := relationship.Between(snapshot, "employees", "missing"); err == nil {
		t.Fatal("missing table accepted")
	}
}

func TestBetweenRejectsUnresolvableForeignKey(t *testing.T) {
	snapshot := schema.Snapshot{Tables: []schema.Table{
		{Name: "parent", Columns: []schema.Column{{Name: "id"}}},
		{Name: "child", Columns: []schema.Column{{Name: "parent_id"}}, ForeignKeys: []schema.ForeignKey{
			{ReferencedTable: "parent", Columns: []schema.ForeignKeyColumn{{From: "parent_id"}}},
		}},
	}}
	if candidates, err := relationship.Between(snapshot, "child", "parent"); err == nil || candidates != nil {
		t.Fatalf("unresolvable key = %#v, %v", candidates, err)
	}
}
