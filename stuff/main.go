// stuff store. Backed by a database.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
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

	// Iterate through all elements.
	IterateAll(func(comp *Component) bool)
}

var wantTimings = flag.Bool("want-timings", false, "Print processing timings.")

func ElapsedPrint(msg string, start time.Time) {
	if *wantTimings {
		log.Printf("%s took %s", msg, time.Since(start))
	}
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
	templateDir := flag.String("templatedir", "./template", "Base-Directory with templates")
	cacheTemplates := flag.Bool("cache-templates", true,
		"Cache templates. False for online editing while development.")
	staticResource := flag.String("staticdir", "static",
		"Directory with static resources")
	port := flag.Int("port", 2000, "Port to serve from")
	dbFile := flag.String("dbfile", "stuff-database.db", "SQLite database file")
	logfile := flag.String("logfile", "", "Logfile to write interesting events")
	do_cleanup := flag.Bool("cleanup-db", false, "Cleanup run of database")
	permitted_nets := flag.String("edit-permission-nets", "", "Comma separated list of networks (CIDR format IP-Addr/network) that are allowed to edit content")
	site_name := flag.String("site-name", "", "Site-name, in particular needed for SSL")

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
	} else {
		log.Printf("Implicitly creating new database file from --dbfile=%s", *dbFile)
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
					cleanupComponent(c)
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

	templates := NewTemplateRenderer(*templateDir, *cacheTemplates)
	AddImageHandler(store, templates, *imageDir, *staticResource)
	AddFormHandler(store, templates, *imageDir, edit_nets)
	AddSearchHandler(store, templates, *imageDir)
	AddStatusHandler(store, templates, *imageDir)
	AddSitemapHandler(store, *site_name)

	log.Printf("Listening on :%d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))

	var block_forever chan bool
	<-block_forever
}
