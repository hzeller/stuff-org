package main

import (
	"strconv"
	"testing"
)

func ExpectTrue(t *testing.T, condition bool, message string) {
	if !condition {
		t.Errorf("Expected to succeed, but didn't: %s", message)
	}
}

func TestBasicStore(t *testing.T) {
	store := NewInMemoryStore()

	ExpectTrue(t, store.FindById(1) == nil, "Expected id:1 not to exist.")

	// Crete record 1, set description
	store.EditRecord(1, func(c *Component) bool {
		c.description = "foo"
		return true
	})

	ExpectTrue(t, store.FindById(1) != nil, "Expected id:1 to exist now.")

	// Edit it, but decide not to proceed
	store.EditRecord(1, func(c *Component) bool {
		ExpectTrue(t, c.description == "foo", "Initial value set")
		c.description = "bar"
		return false // don't commit
	})
	store.EditRecord(1, func(c *Component) bool {
		ExpectTrue(t, c.description == "foo", "Unchanged in second tx")
		return false
	})

	// Now change it
	store.EditRecord(1, func(c *Component) bool {
		c.description = "bar"
		return true
	})
}

func TestBasicMatching(t *testing.T) {
	store := NewInMemoryStore()
	store.EditRecord(1, func(c *Component) bool {
		c.value = "foo" // Value: pretty high score
		return true
	})
	store.EditRecord(2, func(c *Component) bool {
		c.description = "barfoo" // in description, but hidden
		return true
	})
	store.EditRecord(3, func(c *Component) bool {
		c.description = "foo" // in description and first
		return true
	})
	store.EditRecord(4, func(c *Component) bool {
		c.value = "something different"
		return true
	})

	ExpectTrue(t, len(store.Search("nomatch")) == 0, "Search with unexpected result")

	result := store.Search("foo")
	ExpectTrue(t, len(result) == 3, "Unexpected result count "+
		strconv.Itoa(len(result)))
	ExpectTrue(t, result[0].id == 1, "Seq 1 unexpected")
	ExpectTrue(t, result[1].id == 3, "Seq 2 unexpected")
	ExpectTrue(t, result[2].id == 2, "Seq 3 unexpected")
}
