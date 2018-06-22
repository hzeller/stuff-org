package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	kStatusPage = "/status"
	kApiStatus = "/api/status"
	kApiStatusDefaultOffset = 0
	kApiStatusDefaultLimit = 100
)

type StatusHandler struct {
	store    StuffStore
	template *TemplateRenderer
	imgPath  string
}

func AddStatusHandler(store StuffStore, template *TemplateRenderer, imgPath string) {
	handler := &StatusHandler{
		store:    store,
		template: template,
		imgPath:  imgPath,
	}
	http.Handle(kStatusPage, handler)
	http.Handle(kApiStatus, handler)
}

type StatusItem struct {
	Number     int    `json:"number"`
	Status     string `json:"status"`
	Separator  int    `json:"separator,omitempty"`
	HasPicture bool   `json:"haspicture"`
}
type StatusPage struct {
	Items []StatusItem
}

type JsonStatus struct {
	StatusItem
}
type JsonApiStatusResult struct {
	Directlink string  `json:"link"`
	Offset int         `json:"offset"`
	Limit int          `json:"limit"`
	Items []JsonStatus `json:"status"`
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

func (h *StatusHandler) ServeHTTP(out http.ResponseWriter, req *http.Request) {
	if strings.HasPrefix(req.URL.Path, kApiStatus) {
		h.apiStatus(out, req)
	} else {
		current_edit_id := -1
		if cookie, err := req.Cookie("last-edit"); err == nil {
			current_edit_id, _ = strconv.Atoi(cookie.Value)
		}
		defer ElapsedPrint("Show status", time.Now())
		out.Header().Set("Content-Type", "text/html; charset=utf-8")
		maxStatus := 2100
		page := &StatusPage{
			Items: make([]StatusItem, maxStatus),
		}
		for i := 0; i < maxStatus; i++ {
			fillStatusItem(h.store, h.imgPath, i, &page.Items[i])
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
		h.template.Render(out, "status-table.html", page)
	}
}

// Similarly to the Search API, gather the status data and present it in an JSON endpoint.
func (h *StatusHandler) apiStatus(out http.ResponseWriter, r *http.Request) {
	rawOffset := r.FormValue("offset")
	rawLimit := r.FormValue("limit")
	offset := kApiStatusDefaultOffset
	limit := kApiStatusDefaultLimit
	maxStatus := 2100

	//Input validation, restrict inputs from 0 to maxStatus
	if rawOffset != "" {
		parsed_offset, err := strconv.Atoi(rawOffset)
		if err != nil || parsed_offset < 0 {
			offset = kApiStatusDefaultOffset
		} else {
			offset = parsed_offset
		}
	}
	if rawLimit != "" {
		parsed_limit, err := strconv.Atoi(rawLimit)
		if err != nil || parsed_limit < 0 {
			limit = kApiStatusDefaultLimit
		} else {
			limit = parsed_limit
		}
	}
	if offset + limit > maxStatus {
		offset, limit = 0, maxStatus
	}

	out.Header().Set("Cache-Control", "max-age=10")
	out.Header().Set("Content-Type", "application/json")

	page := &StatusPage{
		Items: make([]StatusItem, limit),
	}

	for i := offset; i < offset + limit; i++ {
		fillStatusItem(h.store, h.imgPath, i, &page.Items[i - offset])
	}

	jsonResult := &JsonApiStatusResult{
		Directlink: encodeUriComponent(fmt.Sprintf("/status?offset=%d&limit=%d", offset, limit)),
		Offset:     offset,
		Limit:      limit,
		Items:      make([]JsonStatus, limit),
	}

	for i := 0; i < limit; i++ {
		jsonResult.Items[i].StatusItem = page.Items[i]
	}

	json, _ := json.MarshalIndent(jsonResult, "", "  ")
	out.Write(json)
}
