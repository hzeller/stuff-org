package main

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"strconv"
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

func entryFormHandler(store StuffStore, imageDir string,
	w http.ResponseWriter, r *http.Request) {
	store_id, _ := strconv.Atoi(r.FormValue("store_id"))
	var next_id int
	if r.FormValue("id") != r.FormValue("store_id") {
		// The ID field was edited. Use that as the next ID
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

	// -- Populate page.
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

	// Show status for 10 entries around
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

	// We are not using any web-framework and want to keep track of session
	// cookies. Simply a barebone, state-less web app: use plain cookies.
	w.Header().Set("Set-Cookie", fmt.Sprintf("last-edit=%d", id))
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	zipped := gzip.NewWriter(w)
	renderTemplate(zipped, "form-template", page)
	zipped.Close()
}
