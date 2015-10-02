// stuff store. Backed by a database.
package main

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Component struct {
	Id            int    `json:"id"`
	Value         string `json:"value"`
	Category      string `json:"category"`
	Description   string `json:"description"`
	Quantity      string `json:"quantity"` // at this point just a string.
	Notes         string `json:"notes"`
	Datasheet_url string `json:"datasheet_url"`
	Drawersize    int    `json:"drawersize"`
	Footprint     string `json:"footprint"`
}

// Some useful pre-defined set of categories
var available_category []string = []string{
	"Resistor", "Potentiometer", "R-Network",
	"Capacitor (C)", "Aluminum Cap", "Inductor (L)",
	"Diode (D)", "Power Diode", "LED",
	"Transistor", "Mosfet", "IGBT",
	"Integrated Circuit (IC)", "IC Analog", "IC Digital",
	"Connector", "Socket", "Switch",
	"Fuse", "Mounting", "Heat Sink",
	"Microphone", "Transformer",
}

// Modify a user pointer. Returns 'true' if the changes should be commited.
type ModifyFun func(comp *Component) bool

type StuffStore interface {
	// Find a component by its ID. Returns nil if it does not exist. Don't
	// modify the returned pointer.
	FindById(id int) *Component

	// Edit record of given ID. If ID is new, it is inserted and an empty
	// record returned to be edited.
	// Returns if record has been saved, possibly with message.
	EditRecord(id int, updater ModifyFun) (bool, string)

	// Given a search term, returns all the components that match, ordered
	// by some internal scoring system. Don't modify the returned objects!
	Search(search_term string) []*Component
}

var wantTimings = flag.Bool("want-timings", false, "Print processing timings.")

func ElapsedPrint(msg string, start time.Time) {
	if *wantTimings {
		log.Printf("%s took %s", msg, time.Since(start))
	}
}

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

var cache_templates = flag.Bool("cache-templates", true,
	"Cache templates. False for online editing.")
var templates = template.Must(template.ParseFiles(
	"template/form-template.html",
	"template/status-table.html"))

// for now, render templates directly to easier edit them.
func renderTemplate(w io.Writer, tmpl string, p interface{}) {
	var err error
	template_name := tmpl + ".html"
	if *cache_templates {
		err = templates.ExecuteTemplate(w, template_name, p)
	} else {
		t, err := template.ParseFiles("template/" + template_name)
		if err != nil {
			log.Printf("Template broken %s", err)
			return
		}
		err = t.Execute(w, p)
	}
	if err != nil {
		log.Print("Template broken")
	}
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
	} else {
		// Regular submit. Jump to next
		next_id = store_id + 1
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
			json, _ := json.Marshal(fromForm)
			log.Printf("STORE %s", json)
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
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	zipped := gzip.NewWriter(w)
	renderTemplate(zipped, "form-template", page)
	zipped.Close()
}

func imageServe(prefix_len int, imgPath string, fallbackPath string,
	out http.ResponseWriter, r *http.Request) {
	defer ElapsedPrint("Image serve", time.Now())
	path := r.URL.Path[prefix_len:]
	content, _ := ioutil.ReadFile(imgPath + "/" + path)
	if content == nil && fallbackPath != "" {
		content, _ = ioutil.ReadFile(fallbackPath + "/fallback.jpg")
	}
	out.Header().Set("Cache-Control", "max-age=900")
	switch {
	case strings.HasSuffix(path, ".png"):
		out.Header().Set("Content-Type", "image/png")
	default:
		out.Header().Set("Content-Type", "image/jpg")
	}

	out.Write(content)
	return
}

type JsonSearchResultRecord struct {
	Id    int    `json:"id"`
	Label string `json:"txt"`
}

type JsonSearchResult struct {
	Count int                      `json:"count"`
	Info  string                   `json:"info"`
	Items []JsonSearchResultRecord `json:"items"`
}

func apiSearch(store StuffStore, out http.ResponseWriter, r *http.Request) {
	defer ElapsedPrint("Query", time.Now())
	// Allow very brief caching, so that editing the query does not
	// necessarily has to trigger a new server roundtrip.
	out.Header().Set("Cache-Control", "max-age=10")
	query := r.FormValue("q")
	if query == "" {
		out.Write([]byte("{\"count\":0, \"info\":\"\", \"items\":[]}"))
		return
	}
	start := time.Now()
	searchResults := store.Search(query)
	elapsed := time.Now().Sub(start)
	elapsed = time.Microsecond * ((elapsed + time.Microsecond/2) / time.Microsecond)
	outlen := 24 // Limit max output
	if len(searchResults) < outlen {
		outlen = len(searchResults)
	}
	jsonResult := &JsonSearchResult{
		Count: len(searchResults),
		Info:  fmt.Sprintf("%d results (%s)", len(searchResults), elapsed),
		Items: make([]JsonSearchResultRecord, outlen),
	}

	for i := 0; i < outlen; i++ {
		var c = searchResults[i]
		jsonResult.Items[i].Id = c.Id
		jsonResult.Items[i].Label = "<b>" + html.EscapeString(c.Value) + "</b> " +
			html.EscapeString(c.Description)
	}
	json, _ := json.Marshal(jsonResult)
	out.Write(json)
}

func stuffStoreRoot(out http.ResponseWriter, r *http.Request) {
	http.Redirect(out, r, "/form", 302)
}
func search(out http.ResponseWriter, r *http.Request) {
	out.Header().Set("Content-Type", "text/html; charset=utf-8")
	content, _ := ioutil.ReadFile("template/search-result.html")
	out.Write(content)
}

type StatusItem struct {
	Number     int
	Status     string
	Separator  int
	HasPicture bool
	Highlight  bool
}
type StatusPage struct {
	Items []StatusItem
}

func fillStatusItem(store StuffStore, imageDir string, id int, item *StatusItem) {
	comp := store.FindById(id)
	item.Number = id
	if comp != nil {
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
	} else {
		item.Status = "missing"
	}
	if _, err := os.Stat(fmt.Sprintf("%s/%d.jpg", imageDir, id)); err == nil {
		item.HasPicture = true
	}

}

func listStatus(store StuffStore, imageDir string, out http.ResponseWriter, r *http.Request) {
	defer ElapsedPrint("Show status", time.Now())
	out.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := &StatusPage{
		Items: make([]StatusItem, 1800),
	}
	for i := 0; i < 1800; i++ {
		fillStatusItem(store, imageDir, i, &page.Items[i])
		// Zero is a special case that we handle differently in template.
		if i > 0 {
			if i%100 == 0 {
				page.Items[i].Separator = 2
			} else if i%10 == 0 {
				page.Items[i].Separator = 1
			}
		}
	}
	renderTemplate(out, "status-table", page)
}

func main() {
	imageDir := flag.String("imagedir", "img-srv", "Directory with component images")
	staticResource := flag.String("staticdir", "static",
		"Directory with static resources")
	port := flag.Int("port", 2000, "Port to serve from")
	dbFile := flag.String("dbfile", "stuff-database.db", "SQLite database file")
	logfile := flag.String("logfile", "", "Logfile to write interesting events")

	flag.Parse()

	if *logfile != "" {
		f, err := os.OpenFile(*logfile,
			os.O_RDWR|os.O_CREATE|os.O_APPEND,
			0644)
		if err != nil {
			log.Fatalf("error opening file: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	is_dbfilenew := true
	if _, err := os.Stat(*dbFile); err == nil {
		is_dbfilenew = false
	}

	db, err := sql.Open("sqlite3", *dbFile)
	if err != nil {
		log.Fatal(err)
	}

	var store StuffStore
	store, err = NewDBBackend(db, is_dbfilenew)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/img/", func(w http.ResponseWriter, r *http.Request) {
		imageServe(len("/img/"), *imageDir, *staticResource, w, r)
	})
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		imageServe(len("/static/"), *staticResource, "", w, r)
	})

	http.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		entryFormHandler(store, *imageDir, w, r)
	})

	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		search(w, r)
	})

	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		apiSearch(store, w, r)
	})
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		listStatus(store, *imageDir, w, r)
	})
	http.HandleFunc("/", stuffStoreRoot)

	log.Printf("Listening on :%d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))

	var block_forever chan bool
	<-block_forever
}
