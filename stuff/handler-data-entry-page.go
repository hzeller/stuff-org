package main

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// First, very crude version of radio button selecions
type Selection struct {
	Value        string
	IsSelected   bool
	AddSeparator bool
}
type FormPage struct {
	Component // All these values are shown in the form

	// Category choice.
	CatChoice    []Selection
	CatFallback  Selection
	CategoryText string

	// Additional stuff
	Msg    string // Feedback for user
	PrevId int    // For browsing
	NextId int

	HundredGroup int
	Status       []StatusItem
}

// -- TODO: For cleanup, we need some kind of category-aware plugin structure.

func cleanupResistor(component *Component) {
	optional_percent, _ := regexp.Compile(`,?\s*((0?.)?\d+\%)`)
	if match := optional_percent.FindStringSubmatch(component.Value); match != nil {
		component.Description = match[1] + " " + component.Description
		component.Value = optional_percent.ReplaceAllString(component.Value, "")
	}
	optional_ohm, _ := regexp.Compile(`(?i)\s*ohm`)
	component.Value = optional_ohm.ReplaceAllString(component.Value, "")

	// Upper-case kilo at end or with spaces before are replaced
	// with simple 'k'.
	spaced_upper_kilo, _ := regexp.Compile(`(?i)\s*k$`)
	component.Value = spaced_upper_kilo.ReplaceAllString(component.Value, "k")

	component.Description = cleanString(component.Description)
	component.Value = cleanString(component.Value)
}

func cleanString(input string) string {
	result := strings.TrimSpace(input)
	return strings.Replace(result, "\r\n", "\n", -1)
}

func cleanupCompoent(component *Component) {
	component.Value = cleanString(component.Value)
	component.Category = cleanString(component.Category)
	component.Description = cleanString(component.Description)
	component.Quantity = cleanString(component.Quantity)
	component.Notes = cleanString(component.Notes)
	component.Datasheet_url = cleanString(component.Datasheet_url)
	component.Footprint = cleanString(component.Footprint)

	// We should have pluggable cleanup modules per category. For
	// now just a quick hack.
	switch component.Category {
	case "Resistor":
		cleanupResistor(component)
	}
}

func entryFormHandler(store StuffStore, imageDir string,
	w http.ResponseWriter, r *http.Request) {

	// Look at the request and see what we need to display,
	// and if we have to store something.

	// Store-ID is a hidden field and only set if the form
	// is submitted.
	store_id, _ := strconv.Atoi(r.FormValue("store_id"))
	var next_id int
	if r.FormValue("id") != r.FormValue("store_id") {
		// The ID field was edited. Use that as the next ID the
		// user wants to jump to.
		next_id, _ = strconv.Atoi(r.FormValue("id"))
	} else if r.FormValue("nav_id") != "" {
		// Use the navigation buttons to choose next ID.
		next_id, _ = strconv.Atoi(r.FormValue("nav_id"))
	} else if store_id > 0 {
		// Regular submit. Jump to next
		next_id = store_id + 1
	} else if cookie, err := r.Cookie("last-edit"); err == nil {
		// Last straw: what we remember from last edit.
		next_id, _ = strconv.Atoi(cookie.Value)
	}

	requestStore := r.FormValue("store_id") != ""
	msg := ""
	page := &FormPage{}

	defer ElapsedPrint("Form action", time.Now())

	if requestStore {
		drawersize, _ := strconv.Atoi(r.FormValue("drawersize"))
		fromForm := Component{
			Id:            store_id,
			Value:         r.FormValue("value"),
			Description:   r.FormValue("description"),
			Notes:         r.FormValue("notes"),
			Quantity:      r.FormValue("quantity"),
			Datasheet_url: r.FormValue("datasheet"),
			Drawersize:    drawersize,
			Footprint:     r.FormValue("footprint"),
		}
		// If there only was a ?: operator ...
		if r.FormValue("category_select") == "-" {
			fromForm.Category = r.FormValue("category_txt")
		} else {
			fromForm.Category = r.FormValue("category_select")
		}

		cleanupCompoent(&fromForm)

		was_stored, store_msg := store.EditRecord(store_id, func(comp *Component) bool {
			*comp = fromForm
			return true
		})
		if was_stored {
			msg = fmt.Sprintf("Stored item %d; Proceed to %d", store_id, next_id)
		} else {
			msg = fmt.Sprintf("Item %d (%s); Proceed to %d", store_id, store_msg, next_id)
		}
	} else {
		msg = "Browse item " + fmt.Sprintf("%d", next_id)
	}

	// -- Populate form relevant fields.
	id := next_id
	page.Id = id
	if id > 0 {
		page.PrevId = id - 1
	}
	page.NextId = id + 1
	page.HundredGroup = (id / 100) * 100

	currentItem := store.FindById(id)
	if currentItem != nil {
		page.Component = *currentItem
	} else {
		msg = msg + fmt.Sprintf(" (%d: New item)", id)
	}

	page.CatChoice = make([]Selection, len(available_category))
	anySelected := false
	for i, val := range available_category {
		thisSelected := page.Component.Category == val
		anySelected = anySelected || thisSelected
		page.CatChoice[i] = Selection{
			Value:        val,
			IsSelected:   thisSelected,
			AddSeparator: i%3 == 0}
	}
	page.CatFallback = Selection{
		Value:      "-",
		IsSelected: !anySelected}
	if !anySelected {
		page.CategoryText = page.Component.Category
	}
	page.Msg = msg

	// -- Populate status of fields in current block of 10
	page.Status = make([]StatusItem, 12)
	startStatusId := (id/10)*10 - 1
	if startStatusId <= 0 {
		startStatusId = 0
	}
	for i := 0; i < 12; i++ {
		fillStatusItem(store, imageDir, i+startStatusId, &page.Status[i])
		if i+startStatusId == id {
			page.Status[i].Status = page.Status[i].Status + " selstatus"
		}
	}

	// -- Output
	// We are not using any web-framework or want to keep track of session
	// cookies. Simply a barebone, state-less web app: use plain cookies.
	w.Header().Set("Set-Cookie", fmt.Sprintf("last-edit=%d", id))
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	zipped := gzip.NewWriter(w)
	renderTemplate(zipped, "form-template.html", page)
	zipped.Close()
}

func relatedComponentSetOperations(store StuffStore,
	out http.ResponseWriter, r *http.Request) {
	switch r.FormValue("op") {
	case "html":
		relatedComponentSetHtml(store, out, r)
	case "join":
		relatedComponentSetJoin(store, out, r)
	case "remove":
		relatedComponentSetRemove(store, out, r)
	}
}

func relatedComponentSetJoin(store StuffStore,
	out http.ResponseWriter, r *http.Request) {
	comp, err := strconv.Atoi(r.FormValue("comp"))
	if err != nil {
		return
	}
	set, err := strconv.Atoi(r.FormValue("set"))
	if err != nil {
		return
	}
	store.JoinSet(comp, set)
	relatedComponentSetHtml(store, out, r)
}

func relatedComponentSetRemove(store StuffStore,
	out http.ResponseWriter, r *http.Request) {
	comp, err := strconv.Atoi(r.FormValue("comp"))
	if err != nil {
		return
	}
	store.LeaveSet(comp)
	relatedComponentSetHtml(store, out, r)
}

type EquivalenceSet struct {
	Id    int
	Items []*Component
}
type EquivalenceSetList struct {
	HighlightComp int
	Message       string
	Sets          []*EquivalenceSet
}

func relatedComponentSetHtml(store StuffStore,
	out http.ResponseWriter, r *http.Request) {
	comp_id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		return
	}
	page := &EquivalenceSetList{
		HighlightComp: comp_id,
		Sets:          make([]*EquivalenceSet, 0, 0),
	}
	var current_set *EquivalenceSet = nil
	components := store.MatchingEquivSetForComponent(comp_id)
	switch len(components) {
	case 0:
		page.Message = "No Value or Category set"
	case 1:
		page.Message = "Only one component with this Category/Name"
	default:
		page.Message = "Organize matching components into same virtual drawer (drag'n drop)"
	}

	for _, c := range components {
		if current_set != nil && c.Equiv_set != current_set.Id {
			current_set = nil
		}

		if current_set == nil {
			current_set = &EquivalenceSet{
				Id:    c.Equiv_set,
				Items: make([]*Component, 0, 5),
			}
			page.Sets = append(page.Sets, current_set)
		}
		current_set.Items = append(current_set.Items, c)
	}
	renderTemplate(out, "set-drag-drop.html", page)
}
