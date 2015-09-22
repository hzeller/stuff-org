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
)

type Component struct {
	id            int
	value         string
	category      string
	description   string
	quantity      string // at this point just a string.
	notes         string
	datasheet_url string
	drawersize    int
	// The follwing are not used yet.
	//vendor        string
	//auto_notes    string
	//footprint     string
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
	return 1*StringScore(term, c.category) +
		3*StringScore(term, c.value) +
		2*StringScore(term, c.description)

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

type FormPage struct {
	Msg         string
	Id          string
	PrevId      string
	NextId      string
	Category    string
	Value       string
	Description string
	Notes       string
	Quantity    string
	Datasheet   string
	Drawersize  int
}

// for now, render templates directly to easier edit them.
func renderTemplate(w http.ResponseWriter, tmpl string, p *FormPage) {
	t, err := template.ParseFiles("template/" + tmpl + ".html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = t.Execute(w, p)
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
		success, err := store.EditRecord(id, func(comp *Component) bool {
			comp.category = category
			comp.value = r.FormValue("value")
			comp.description = r.FormValue("description")
			comp.notes = r.FormValue("notes")
			comp.quantity = r.FormValue("quantity")
			comp.datasheet_url = r.FormValue("datasheet")
			comp.drawersize, _ = strconv.Atoi(r.FormValue("drawersize"))
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

	page.Id = strconv.Itoa(id)
	if id > 0 {
		page.PrevId = strconv.Itoa(id - 1)
	}
	page.NextId = strconv.Itoa(id + 1)
	currentItem := store.FindById(id)
	if currentItem != nil {
		page.Category = currentItem.category
		page.Value = currentItem.value
		page.Description = currentItem.description
		page.Notes = currentItem.notes
		page.Quantity = currentItem.quantity
		page.Datasheet = currentItem.datasheet_url
		page.Drawersize = currentItem.drawersize
	} else {
		msg = "Edit new item " + fmt.Sprintf("%d", id)
	}

	page.Msg = msg
	renderTemplate(w, "form-template", page)
}

func imageServe(prefix_len int, imgPath string, fallbackPath string,
	out http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[prefix_len:]
	content, _ := ioutil.ReadFile(imgPath + "/" + path)
	if content == nil && fallbackPath != "" {
		content, _ = ioutil.ReadFile(fallbackPath + "/fallback.jpg")
	}
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

	//store := NewInMemoryStore()
	store, err := NewDBBackend(db)
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
		entryFormHandler(store, w, r)
	})

	http.HandleFunc("/", stuffStoreRoot)

	log.Printf("Listening on :%d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))

	var block_forever chan bool
	<-block_forever
}
