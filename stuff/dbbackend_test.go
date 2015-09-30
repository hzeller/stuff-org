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
