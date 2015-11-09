package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type StatusItem struct {
	Number     int
	Status     string
	Separator  int
	HasPicture bool
}
type StatusPage struct {
	Items []StatusItem
}

func fillStatusItem(store StuffStore, imageDir string, id int, item *StatusItem) {
	comp := store.FindById(id)
	item.Number = id
	if comp != nil {
		// Ad-hoc categorization...
		count := 0
		if comp.Category != "" {
			count++
		}
		if comp.Value != "" {
			count++
		}
		// Description should be set. But for simple things such
		// as resistors or capacitors, we see just one value
		// to be sufficient. Totally hacky classification :)
		if comp.Description != "" ||
			(comp.Category == "Resistor" && comp.Value != "") ||
			(comp.Category == "Capacitor (C)" && comp.Value != "") {
			count++
		}
		switch count {
		case 0:
			item.Status = "missing"
		case 1:
			item.Status = "poor"
		case 2:
			item.Status = "fair"
		case 3:
			item.Status = "good"
		}
		if strings.Index(strings.ToLower(comp.Value), "empty") >= 0 ||
			strings.Index(strings.ToLower(comp.Category), "empty") >= 0 {
			item.Status = "empty"
		}
		if strings.Index(strings.ToLower(comp.Category), "mystery") >= 0 ||
			strings.Index(comp.Value, "?") >= 0 {
			item.Status = "mystery"
		}
	} else {
		item.Status = "missing"
	}
	if _, err := os.Stat(fmt.Sprintf("%s/%d.jpg", imageDir, id)); err == nil {
		item.HasPicture = true
	}

}

func showStatusPage(store StuffStore, imageDir string, out http.ResponseWriter, r *http.Request) {
	current_edit_id := -1
	if cookie, err := r.Cookie("last-edit"); err == nil {
		current_edit_id, _ = strconv.Atoi(cookie.Value)
	}
	defer ElapsedPrint("Show status", time.Now())
	out.Header().Set("Content-Type", "text/html; charset=utf-8")
	maxStatus := 2100
	page := &StatusPage{
		Items: make([]StatusItem, maxStatus),
	}
	for i := 0; i < maxStatus; i++ {
		fillStatusItem(store, imageDir, i, &page.Items[i])
		// Zero is a special case that we handle differently in template.
		if i > 0 {
			if i%100 == 0 {
				page.Items[i].Separator = 2
			} else if i%10 == 0 {
				page.Items[i].Separator = 1
			}
		}
		if i == current_edit_id {
			page.Items[i].Status = page.Items[i].Status + " selstatus"
		}

	}
	renderTemplate(out, out.Header(), "status-table.html", page)
}
