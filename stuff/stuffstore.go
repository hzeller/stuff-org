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
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Component struct {
	id          int
	value       string
	category    string
	description string
	// The follwing are not used yet.
	//notes         string
	//datasheet_url string
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

type DBBackend struct {
	db           *sql.DB
	findById     *sql.Stmt
	insertRecord *sql.Stmt
	updateRecord *sql.Stmt
}

func NewDBBackend(db *sql.DB) (*DBBackend, error) {
	findById, err := db.Prepare("SELECT category, value, description " +
		"FROM component where id=$1")
	if err != nil {
		return nil, err
	}
	insertRecord, err := db.Prepare("INSERT INTO component (id, category, value, description) " +
		"VALUES ($1, $2, $3, $4)")
	if err != nil {
		return nil, err
	}
	updateRecord, err := db.Prepare("UPDATE component SET " +
		"category=$2, value=$3, description=$4 where id=$1 ")
	if err != nil {
		return nil, err
	}

	return &DBBackend{
		db:           db,
		findById:     findById,
		insertRecord: insertRecord,
		updateRecord: updateRecord}, nil
}

func (d *DBBackend) FindById(id int) *Component {
	result := &Component{id: id}
	err := d.findById.QueryRow(id).Scan(&result.category, &result.value, &result.description)
	switch {
	case err == sql.ErrNoRows:
		return nil
	case err != nil:
		log.Fatal(err)
	default:
		return result
	}
	return nil
}

func (d *DBBackend) EditRecord(id int, update ModifyFun) (bool, string) {
	needsInsert := false
	toEdit := d.FindById(id)
	if toEdit == nil {
		needsInsert = true
		toEdit = &Component{id: id}
	}
	if update(toEdit) {
		if toEdit.id != id {
			return false, "ID was modified"
		}
		var err error
		if needsInsert {
			_, err = d.insertRecord.Exec(id, toEdit.category, toEdit.value,
				toEdit.description)
		} else {
			_, err = d.updateRecord.Exec(id, toEdit.category, toEdit.value,
				toEdit.description)
		}
		if err != nil {
			return false, err.Error()
		}
	}
	return true, ""
}

func (d *DBBackend) Search(search_term string) []*Component {
	return nil // not implemented yet.
}

type InMemoryStore struct {
	lock         sync.Mutex
	id2Component map[int]*Component
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		id2Component: make(map[int]*Component),
	}
}
func (s *InMemoryStore) EditRecord(id int, update ModifyFun) (bool, string) {
	var toEdit Component
	s.lock.Lock()
	found := s.id2Component[id]
	if found != nil {
		toEdit = *found
	} else {
		toEdit.id = id
	}
	s.lock.Unlock()
	if update(&toEdit) {
		toEdit.id = id // We don't allow to mess with that one :)
		s.lock.Lock()
		defer s.lock.Unlock()
		if s.id2Component[id] != found {
			return false, "Editing conflict. Discarding this edit. Sorry."
		}
		s.id2Component[id] = &toEdit
		return true, ""
	}
	return true, ""
}

func (s *InMemoryStore) FindById(id int) *Component {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.id2Component[id]
}

type ScoredComponent struct {
	score float32
	comp  *Component
}
type ScoreList []*ScoredComponent

func (s ScoreList) Len() int {
	return len(s)
}
func (s ScoreList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ScoreList) Less(i, j int) bool {
	// We want to reverse score: highest match first
	return s[i].score > s[j].score
}

func (s *InMemoryStore) Search(search_term string) []*Component {
	s.lock.Lock()
	scoredlist := make(ScoreList, 0, 10)
	for _, comp := range s.id2Component {
		scored := &ScoredComponent{
			score: comp.MatchScore(search_term),
			comp:  comp,
		}
		if scored.score > 0 {
			scoredlist = append(scoredlist, scored)
		}
	}
	s.lock.Unlock()
	sort.Sort(ScoreList(scoredlist))
	result := make([]*Component, len(scoredlist))
	for idx, scomp := range scoredlist {
		result[idx] = scomp.comp
	}
	return result
}

type FormPage struct {
	Msg         string
	Id          string
	PrevId      string
	NextId      string
	Category    string
	Value       string
	Description string
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
		page.Description = currentItem.description
		page.Value = currentItem.value
	} else {
		msg = "Edit new item " + fmt.Sprintf("%d", id)
	}

	page.Msg = msg
	renderTemplate(w, "form-template", page)
}

func imageServe(imgPath string, out http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/img/"):]
	content, _ := ioutil.ReadFile(imgPath + "/" + path)
	if content == nil {
		content, _ = ioutil.ReadFile(imgPath + "/fallback.jpg")
	}
	out.Header()["Content-Type"] = []string{"image/jpeg"}
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
		imageServe(*imageDir, w, r)
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
