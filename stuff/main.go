// stuff store. Backed by a database.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Component struct {
	Id            int
	Value         string
	Category      string
	Description   string
	Quantity      string // at this point just a string.
	Notes         string
	Datasheet_url string
	Drawersize    int
	Footprint     string
	// The follwing are not used yet.
	//vendor        string
	//auto_notes    string
	//footprint     string
}

// Modify a user pointer. Returns 'true' if the changes should be commited.
type ModifyFun func(comp *Component) bool

type StuffStore interface {
	// Find a component by its ID. Returns nil if it does not exist. Don't
	// modify the returned pointer.
	FindById(id int) *Component

	// Edit record of given ID. If ID is new, it is inserted and an empty
	// record returned to be edited. Returns 'true' if transaction has
	// been successfully commited (or aborted by ModifyFun.)
	EditRecord(id int, updater ModifyFun) (bool, string)

	// Given a search term, returns all the components that match, ordered
	// by some internal scoring system. Don't modify the returned objects!
	Search(search_term string) []*Component
}

func StringScore(needle string, haystack string) float32 {
	switch strings.Index(haystack, needle) {
	case -1:
		return 0
	case 0:
		return 2 // start of string, higher match
	default:
		return 1
	}
}

// Matches the component and returns a score
func (c *Component) MatchScore(term string) float32 {
	return 1*StringScore(term, c.Category) +
		3*StringScore(term, c.Value) +
		2*StringScore(term, c.Description)

}

var wantTimings = flag.Bool("want_timings", false, "Print processing timings.")

func ElapsedPrint(msg string, start time.Time) {
	if *wantTimings {
		log.Printf("%s took %s", msg, time.Since(start))
	}
}

type FormPage struct {
	Component // All these values are shown in the form
	// Additional stuff
	Msg    string // Feedback for user
	PrevId int    // For browsing
	NextId int
}

var cache_templates = flag.Bool("cache_templates", true,
	"Cache templates. False for online editing.")
var templates = template.Must(template.ParseFiles("template/form-template.html"))

// for now, render templates directly to easier edit them.
func renderTemplate(w http.ResponseWriter, tmpl string, p *FormPage) {
	var err error
	template_name := tmpl + ".html"
	if *cache_templates {
		err = templates.ExecuteTemplate(w, template_name, p)
	} else {
		t, err := template.ParseFiles("template/" + template_name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = t.Execute(w, p)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func entryFormHandler(store StuffStore, w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.FormValue("id"))
	requestStore := r.FormValue("send") != "" && r.FormValue("orig_id") == r.FormValue("id")
	category := r.FormValue("category")
	msg := ""
	page := &FormPage{}

	if requestStore {
		defer ElapsedPrint("Storing item", time.Now())
	} else {
		defer ElapsedPrint("View item", time.Now())
	}
	if requestStore {
		success, err := store.EditRecord(id, func(comp *Component) bool {
			comp.Category = category
			comp.Value = r.FormValue("value")
			comp.Description = r.FormValue("description")
			comp.Notes = r.FormValue("notes")
			comp.Quantity = r.FormValue("quantity")
			comp.Datasheet_url = r.FormValue("datasheet")
			comp.Drawersize, _ = strconv.Atoi(r.FormValue("drawersize"))
			comp.Footprint = r.FormValue("footprint")
			return true
		})
		if success {
			msg = "Stored item " + fmt.Sprintf("%d", id)
		} else {
			msg = "ERROR STORING STUFF DAMNIT. " + err + fmt.Sprintf("ID=%d", id)
		}
	} else {
		msg = "Edit item " + fmt.Sprintf("%d", id)
	}

	if requestStore {
		id = id + 1 // be helpful and suggest next
	}

	page.Id = id
	if id > 0 {
		page.PrevId = id - 1
	}
	page.NextId = id + 1
	currentItem := store.FindById(id)
	if currentItem != nil {
		page.Component = *currentItem
	} else {
		msg = "Edit new item " + fmt.Sprintf("%d", id)
	}

	page.Msg = msg
	renderTemplate(w, "form-template", page)
}

func imageServe(prefix_len int, imgPath string, fallbackPath string,
	out http.ResponseWriter, r *http.Request) {
	defer ElapsedPrint("Image serve", time.Now())
	path := r.URL.Path[prefix_len:]
	content, _ := ioutil.ReadFile(imgPath + "/" + path)
	if content == nil && fallbackPath != "" {
		content, _ = ioutil.ReadFile(fallbackPath + "/fallback.jpg")
	}
	out.Header()["Cache-Control"] = []string{"max-age=900"}
	switch {
	case strings.HasSuffix(path, ".png"):
		out.Header()["Content-Type"] = []string{"image/png"}
	default:
		out.Header()["Content-Type"] = []string{"image/jpeg"}
	}

	out.Write(content)
	return
}

func stuffStoreRoot(out http.ResponseWriter, r *http.Request) {
	out.Header()["Content-Type"] = []string{"text/html"}
	out.Write([]byte("Welcome to StuffStore. " +
		"Here is an <a href='/form'>input form</a>."))
}

func main() {
	imageDir := flag.String("imagedir", "img-srv", "Directory with images")
	staticResource := flag.String("staticdir", "static", "Directory with static resources")
	port := flag.Int("port", 2000, "Port to serve from")
	dbName := flag.String("db", "stuff", "Database to connect")
	dbUser := flag.String("dbuser", "hzeller", "Database user")
	dbPwd := flag.String("dbpwd", "", "Database password")

	flag.Parse()

	db, err := sql.Open("postgres",
		fmt.Sprintf("user=%s dbname=%s password=%s",
			*dbUser, *dbName, *dbPwd))
	if err != nil {
		log.Fatal(err)
	}

	var store StuffStore
	store, err = NewDBBackend(db)
	if err != nil {
		log.Fatal(err)
	}
	//store = NewInMemoryStore()
	http.HandleFunc("/img/", func(w http.ResponseWriter, r *http.Request) {
		imageServe(len("/img/"), *imageDir, *staticResource, w, r)
	})
	http.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
		imageServe(len("/static/"), *staticResource, "", w, r)
	})

	http.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
		entryFormHandler(store, w, r)
	})

	http.HandleFunc("/", stuffStoreRoot)

	log.Printf("Listening on :%d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))

	var block_forever chan bool
	<-block_forever
}
