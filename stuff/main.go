// stuff store. Backed by a database.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

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
	// Application templates
	"template/form-template.html",
	"template/status-table.html",
	"template/set-drag-drop.html",
	// Templates to create component images
	"template/component/category-Diode.svg",
	"template/component/category-LED.svg",
	"template/component/category-Capacitor.svg",
	// Value rendering of resistors
	"template/component/4-Band_Resistor.svg",
	"template/component/5-Band_Resistor.svg",
	// Some common packages
	"template/component/package-TO-39.svg",
	"template/component/package-TO-220.svg",
	"template/component/package-DIP-14.svg",
	"template/component/package-DIP-16.svg",
	"template/component/package-DIP-28.svg"))

func setContentTypeFromTemplateName(template_name string, header http.Header) {
	switch {
	case strings.HasSuffix(template_name, ".svg"):
		header.Set("Content-Type", "image/svg+xml")
	default:
		header.Set("Content-Type", "text/html; charset=utf-8")
	}
}

// for now, render templates directly to easier edit them.
func renderTemplate(w io.Writer, header http.Header, template_name string, p interface{}) bool {
	var err error
	if *cache_templates {
		template := templates.Lookup(template_name)
		if template == nil {
			return false
		}
		setContentTypeFromTemplateName(template_name, header)
		err = template.Execute(w, p)
	} else {
		t, err := template.ParseFiles("template/" + template_name)
		if err != nil {
			t, err = template.ParseFiles("template/component/" + template_name)
			if err != nil {
				log.Printf("%s: %s", template_name, err)
				return false
			}
		}
		setContentTypeFromTemplateName(template_name, header)
		err = t.Execute(w, p)
	}
	if err != nil {
		log.Printf("Template broken %s", template_name)
		return false
	}
	return true
}

func stuffStoreRoot(out http.ResponseWriter, r *http.Request) {
	http.Redirect(out, r, "/search", 302)
}

func parseAllowedEditorCIDR(allowed string) []*net.IPNet {
	all_allowed := strings.Split(allowed, ",")
	allowed_nets := make([]*net.IPNet, 0, len(all_allowed))
	for i := 0; i < len(all_allowed); i++ {
		if all_allowed[i] == "" {
			continue
		}
		_, net, err := net.ParseCIDR(all_allowed[i])
		if err != nil {
			log.Fatal("--edit-permission-nets: Need IP/Network format: ", err)
		} else {
			allowed_nets = append(allowed_nets, net)
		}
	}
	return allowed_nets
}

func main() {
	imageDir := flag.String("imagedir", "img-srv", "Directory with component images")
	staticResource := flag.String("staticdir", "static",
		"Directory with static resources")
	port := flag.Int("port", 2000, "Port to serve from")
	dbFile := flag.String("dbfile", "stuff-database.db", "SQLite database file")
	logfile := flag.String("logfile", "", "Logfile to write interesting events")
	do_cleanup := flag.Bool("cleanup-db", false, "Cleanup run of database")
	permitted_nets := flag.String("edit-permission-nets", "", "Comma separated list of networks (CIDR format IP-Addr/network) that are allowed to edit content")

	flag.Parse()

	edit_nets := parseAllowedEditorCIDR(*permitted_nets)

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

	// Very crude way to run all the cleanup routines if
	// requested. This is the only thing we do.
	if *do_cleanup {
		for i := 0; i < 3000; i++ {
			if c := store.FindById(i); c != nil {
				store.EditRecord(i, func(c *Component) bool {
					before := *c
					cleanupCompoent(c)
					if *c == before {
						return false
					}
					json, _ := json.Marshal(before)
					log.Printf("----- %s", json)
					return true
				})
			}
		}
		return
	}

	AddImageHandler(store, *imageDir, *staticResource)

	// TODO(hzeller): Now that is clear what we want, these should
	// also become http.Handlers

	http.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		entryFormHandler(store, *imageDir, edit_nets, w, r)
	})
	http.HandleFunc("/api/related-set", func(w http.ResponseWriter, r *http.Request) {
		relatedComponentSetOperations(store, edit_nets, w, r)
	})

	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		showSearchPage(w, r)
	})
	// Pre-formatted for quick page display
	http.HandleFunc("/api/search-formatted", func(w http.ResponseWriter, r *http.Request) {
		apiSearchPageItem(store, w, r)
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
