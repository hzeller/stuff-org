// stuff store. Backed by a database.
package main

import (
	"database/sql"
	"flag"
	"fmt"
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
	Equiv_set     int    `json:"equiv_set"`
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
	"Microphone", "Transformer", "? MYSTERY",
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
	Search(search_term string) []*Component
}

var wantTimings = flag.Bool("want-timings", false, "Print processing timings.")

func ElapsedPrint(msg string, start time.Time) {
	if *wantTimings {
		log.Printf("%s took %s", msg, time.Since(start))
	}
}

var cache_templates = flag.Bool("cache-templates", true,
	"Cache templates. False for online editing.")
var templates = template.Must(template.ParseFiles(
	"template/form-template.html",
	"template/status-table.html",
	"template/set-drag-drop.html",
	"template/4-Band_Resistor.svg",
	"template/5-Band_Resistor.svg"))

// for now, render templates directly to easier edit them.
func renderTemplate(w io.Writer, template_name string, p interface{}) {
	var err error
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

func sendResource(local_path string, fallback_resource string, out http.ResponseWriter) {
	cache_time := 900
	content, _ := ioutil.ReadFile(local_path)
	if content == nil && fallback_resource != "" {
		local_path = fallback_resource
		content, _ = ioutil.ReadFile(local_path)
		cache_time = 10 // fallbacks might change more often.
	}
	out.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d", cache_time))
	switch {
	case strings.HasSuffix(local_path, ".png"):
		out.Header().Set("Content-Type", "image/png")
	case strings.HasSuffix(local_path, ".css"):
		out.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(local_path, ".svg"):
		out.Header().Set("Content-Type", "image/svg+xml")
	default:
		out.Header().Set("Content-Type", "image/jpg")
	}

	out.Write(content)
}

func serveComponentImage(component *Component, category string, value string,
	out http.ResponseWriter) bool {
	// If we got a category string, it takes precedence
	if len(category) == 0 && component != nil {
		category = component.Category
	}
	if category == "Resistor" {
		return serveResistorImage(component, value, out)
	}
	return false
}

func compImageServe(store StuffStore, imgPath string, staticPath string,
	out http.ResponseWriter, r *http.Request) {
	prefix_len := len("/img/")
	requested := r.URL.Path[prefix_len:]
	path := imgPath + "/" + requested + ".jpg"
	if _, err := os.Stat(path); err == nil { // we have an image.
		fmt.Printf("%s: found image", requested)
		sendResource(path, staticPath+"/fallback.jpg", out)
		return
	}
	// No image, but let's see if we can do something from the
	// component
	if comp_id, err := strconv.Atoi(requested); err == nil {
		component := store.FindById(comp_id)
		category := r.FormValue("c") // We also allow these if available
		value := r.FormValue("v")
		if (component != nil || len(category) > 0 || len(value) > 0) &&
			serveComponentImage(component, category, value, out) {
			return
		}
	}
	sendResource(staticPath+"/fallback.jpg", "", out)
}

func staticServe(staticPath string, out http.ResponseWriter, r *http.Request) {
	prefix_len := len("/static/")
	resource := r.URL.Path[prefix_len:]
	sendResource(staticPath+"/"+resource, "", out)
}

func stuffStoreRoot(out http.ResponseWriter, r *http.Request) {
	http.Redirect(out, r, "/form", 302)
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
		compImageServe(store, *imageDir, *staticResource, w, r)
	})
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		staticServe(*staticResource, w, r)
	})

	http.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		entryFormHandler(store, *imageDir, w, r)
	})
	http.HandleFunc("/api/related-set", func(w http.ResponseWriter, r *http.Request) {
		relatedComponentSetOperations(store, w, r)
	})

	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		showSearchPage(w, r)
	})
	http.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		apiSearch(store, w, r)
	})

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		showStatusPage(store, *imageDir, w, r)
	})

	http.HandleFunc("/", stuffStoreRoot)

	log.Printf("Listening on :%d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))

	var block_forever chan bool
	<-block_forever
}
