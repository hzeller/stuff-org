package main

type Component struct {
	Id            int    `json:"id"`
	Equiv_set     int    `json:"equiv_set,omitempty"`
	Value         string `json:"value"`
	Category      string `json:"category"`
	Description   string `json:"description"`
	Quantity      string `json:"quantity"` // at this point just a string.
	Notes         string `json:"notes,omitempty"`
	Datasheet_url string `json:"datasheet_url,omitempty"`
	Drawersize    int    `json:"drawersize,omitempty"`
	Footprint     string `json:"footprint,omitempty"`
}

// Modify a user pointer. Returns 'true' if the changes should be commited.
type ModifyFun func(comp *Component) bool

// Interface to our storage backend.
type StuffStore interface {
	// Find a component by its ID. Returns nil if it does not exist. Don't
	// modify the returned pointer.
	FindById(id int) *Component

	// Edit record of given ID. If ID is new, it is inserted and an empty
	// record returned to be edited.
	// Returns if record has been saved, possibly with message.
	// This does _not_ influence the equivalence set settings, use
	// the JoinSet()/LeaveSet() functions for that.
	EditRecord(id int, updater ModifyFun) (bool, string)

	// Have component with id join set with given ID.
	JoinSet(id int, equiv_set int)

	// Leave any set we are in and go back to the default set
	// (which is equiv_set == id)
	LeaveSet(id int)

	// Get possible matching components of given component,
	// including all the components that are in the sets the matches
	// are in.
	// Ordered by equivalence set, id.
	MatchingEquivSetForComponent(component int) []*Component

	// Given a search term, returns all the components that match, ordered
	// by some internal scoring system. Don't modify the returned objects!
	Search(search_term string) *SearchResult

	// Iterate through all elements.
	IterateAll(func(comp *Component) bool)
}
