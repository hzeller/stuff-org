package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"io/ioutil"
	"log"
	"syscall"
	"testing"
)

func ExpectTrue(t *testing.T, condition bool, message string) {
	if !condition {
		t.Errorf("Expected to succeed, but didn't: %s", message)
	}
}

func TestBasicStore(t *testing.T) {
	dbfile, _ := ioutil.TempFile("", "basic-store")
	defer syscall.Unlink(dbfile.Name())
	db, err := sql.Open("sqlite3", dbfile.Name())
	if err != nil {
		log.Fatal(err)
	}
	store, _ := NewDBBackend(db, true)

	ExpectTrue(t, store.FindById(1) == nil, "Expected id:1 not to exist.")

	// Create record 1, set description
	store.EditRecord(1, func(c *Component) bool {
		c.Description = "foo"
		return true
	})

	ExpectTrue(t, store.FindById(1) != nil, "Expected id:1 to exist now.")

	// Edit it, but decide not to proceed
	store.EditRecord(1, func(c *Component) bool {
		ExpectTrue(t, c.Description == "foo", "Initial value set")
		c.Description = "bar"
		return false // don't commit
	})
	ExpectTrue(t, store.FindById(1).Description == "foo", "Unchanged in second tx")

	// Now change it
	store.EditRecord(1, func(c *Component) bool {
		c.Description = "bar"
		return true
	})
	ExpectTrue(t, store.FindById(1).Description == "bar", "Description change")
}

func TestJoinSets(t *testing.T) {
	dbfile, _ := ioutil.TempFile("", "basic-store")
	defer syscall.Unlink(dbfile.Name())
	db, err := sql.Open("sqlite3", dbfile.Name())
	if err != nil {
		log.Fatal(err)
	}
	store, _ := NewDBBackend(db, true)

	// Three components, each in their own equiv-class
	store.EditRecord(1, func(c *Component) bool { c.Value = "one"; return true })
	store.EditRecord(2, func(c *Component) bool { c.Value = "two"; return true })
	store.EditRecord(3, func(c *Component) bool { c.Value = "three"; return true })

	// Expecting baseline.
	ExpectTrue(t, store.FindById(1).Equiv_set == 1, "#1")
	ExpectTrue(t, store.FindById(2).Equiv_set == 2, "#2")
	ExpectTrue(t, store.FindById(3).Equiv_set == 3, "#3")

	// Component 2 join set 3. Final equivalence-set is lowest
	// id of the result set.
	store.JoinSet(2, 3)
	ExpectTrue(t, store.FindById(1).Equiv_set == 1, "#4")
	ExpectTrue(t, store.FindById(2).Equiv_set == 2, "#5")
	ExpectTrue(t, store.FindById(3).Equiv_set == 2, "#6")

	// Break out article three out of this set.
	store.LeaveSet(3)
	ExpectTrue(t, store.FindById(1).Equiv_set == 1, "#7")
	ExpectTrue(t, store.FindById(2).Equiv_set == 2, "#8")
	ExpectTrue(t, store.FindById(3).Equiv_set == 3, "#9")

	// Join everything together.
	store.JoinSet(3, 1)
	ExpectTrue(t, store.FindById(1).Equiv_set == 1, "#10")
	ExpectTrue(t, store.FindById(2).Equiv_set == 2, "#11")
	ExpectTrue(t, store.FindById(3).Equiv_set == 1, "#12")
	store.JoinSet(2, 1)
	ExpectTrue(t, store.FindById(1).Equiv_set == 1, "#12")
	ExpectTrue(t, store.FindById(2).Equiv_set == 1, "#13")
	ExpectTrue(t, store.FindById(3).Equiv_set == 1, "#14")

	// Lowest component leaving the set leaves the equivalence set
	// at the lowest of the remaining.
	store.LeaveSet(1)
	ExpectTrue(t, store.FindById(1).Equiv_set == 1, "#15")
	ExpectTrue(t, store.FindById(2).Equiv_set == 2, "#16")
	ExpectTrue(t, store.FindById(3).Equiv_set == 2, "#17")
}
