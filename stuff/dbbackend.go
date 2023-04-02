package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"
)

// Initial phase: while collecting the raw information, a single flat table
// is sufficient.
var create_schema string = `
create table component (
       id            int           constraint pk_component primary key,
       equiv_set     int not null, -- equivlennce set; points to lowest
                                   -- component in set.
       category      varchar(40),  -- should be some foreign key
       value         varchar(80),  -- identifying the component value
       description   text,         -- additional information
       notes         text,         -- user notes, can contain hashtags.
       datasheet_url text,         -- data sheet URL if available
       vendor        varchar(30),  -- should be foreign key
       auto_notes    text,         -- auto generated notes, might aid in search
       footprint     varchar(30),
       quantity      varchar(5),   -- Initially text to allow freeform e.g '< 50'
       drawersize    int,          -- 0=small, 1=medium, 2=large

       created timestamp,
       updated timestamp,

       -- also, we need the following eventually
       -- labeltext, drawer-type, location. Several of these should have foreign keys.

      foreign key(equiv_set) references component(id)
);
`

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	} else {
		return &s
	}
}
func emptyIfNull(s *string) string {
	if s == nil {
		return ""
	} else {
		return *s
	}
}

func row2Component(row *sql.Rows) (*Component, error) {
	type ReadRecord struct {
		id          int
		equiv_set   int
		category    *string
		value       *string
		description *string
		notes       *string
		quantity    *string
		datasheet   *string
		drawersize  *int
		footprint   *string
	}
	rec := &ReadRecord{}
	err := row.Scan(&rec.id, &rec.category, &rec.value,
		&rec.description, &rec.notes, &rec.quantity, &rec.datasheet,
		&rec.drawersize, &rec.footprint, &rec.equiv_set)
	drawersize := 0
	if rec.drawersize != nil {
		drawersize = *rec.drawersize
	}
	switch {
	case err == sql.ErrNoRows:
		return nil, nil // no rows are ok error.
	case err != nil:
		log.Fatal(err)
	default:
		result := &Component{
			Id:            rec.id,
			Equiv_set:     rec.equiv_set,
			Category:      emptyIfNull(rec.category),
			Value:         emptyIfNull(rec.value),
			Description:   emptyIfNull(rec.description),
			Notes:         emptyIfNull(rec.notes),
			Quantity:      emptyIfNull(rec.quantity),
			Datasheet_url: emptyIfNull(rec.datasheet),
			Drawersize:    drawersize,
			Footprint:     emptyIfNull(rec.footprint),
		}
		return result, nil
	}
	return nil, nil
}

type DBBackend struct {
	db            *sql.DB
	findById      *sql.Stmt
	insertRecord  *sql.Stmt
	updateRecord  *sql.Stmt
	joinSet       *sql.Stmt
	leaveSet      *sql.Stmt
	findEquivById *sql.Stmt
	selectAll     *sql.Stmt
	fts           *FulltextSearch
}

func NewDBBackend(db *sql.DB, create_tables bool) (*DBBackend, error) {
	if create_tables {
		_, err := db.Exec(create_schema)
		if err != nil {
			log.Fatal(err)
		}
	}
	// All the fields in a component.
	all_fields := "category, value, description, notes, quantity, datasheet_url,drawersize,footprint,equiv_set"
	findById, err := db.Prepare("SELECT id, " + all_fields + " FROM component where id=$1")
	if err != nil {
		return nil, err
	}

	// For writing a component, we need insert and update. In the full
	// component update, we explicitly do not want to update the
	// membership to the set, so we don't touch these fields.
	insertRecord, err := db.Prepare("INSERT INTO component (id, created, updated, " + all_fields + ") " +
		" VALUES (?1, ?2, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?1)")
	if err != nil {
		return nil, err
	}
	updateRecord, err := db.Prepare("UPDATE component SET " +
		"updated=?2, category=?3, value=?4, description=?5, notes=?6, quantity=?7, datasheet_url=?8, drawersize=?9, footprint=?10 WHERE id=?1")
	if err != nil {
		return nil, err
	}

	// Statements for set operations.
	joinSet, err := db.Prepare("UPDATE component SET equiv_set = MIN(?1, ?2) WHERE equiv_set = ?2 OR id = ?1")
	if err != nil {
		return nil, err
	}

	leaveSet, err := db.Prepare("UPDATE component SET equiv_set = CASE WHEN id = ?1 THEN ?1 ELSE (select min(id) from component where equiv_set = ?2 and id != ?1) end where equiv_set = ?2")
	if err != nil {
		return nil, err
	}

	// We want all articles that match the same (category, name), but also
	// all that are in the sets that are covered in any set the matching
	// components are in.
	// Todo: maybe in-memory and more lenient way to match values
	findEquivById, err := db.Prepare(`
	    SELECT id, ` + all_fields + ` FROM component where equiv_set in
	        (select c2.equiv_set from component c1, component c2
	          where lower(c1.value) = lower(c2.value)
	            and c1.category = c2.category and c1.id = ?1)
	    ORDER BY equiv_set, id`)
	if err != nil {
		return nil, err
	}

	selectAll, err := db.Prepare("SELECT id, " + all_fields + " FROM component ORDER BY id")
	if err != nil {
		return nil, err
	}
	// Populate fts with existing components.
	fts := NewFulltextSearch()
	rows, _ := selectAll.Query()
	count := 0
	for rows != nil && rows.Next() {
		c, _ := row2Component(rows)
		fts.Update(c)
		count++
	}
	rows.Close()

	log.Printf("Prepopulated full text search with %d items", count)
	return &DBBackend{
		db:            db,
		findById:      findById,
		insertRecord:  insertRecord,
		updateRecord:  updateRecord,
		joinSet:       joinSet,
		leaveSet:      leaveSet,
		findEquivById: findEquivById,
		selectAll:     selectAll,
		fts:           fts}, nil
}

func (d *DBBackend) FindById(id int) *Component {
	rows, _ := d.findById.Query(id)
	if rows != nil {
		defer rows.Close()
		if rows.Next() {
			c, _ := row2Component(rows)
			return c
		}
	}
	return nil
}

func (d *DBBackend) IterateAll(callback func(comp *Component) bool) {
	rows, _ := d.selectAll.Query()
	for rows != nil && rows.Next() {
		c, _ := row2Component(rows)
		if !callback(c) {
			break
		}
	}
	rows.Close()
}

func (d *DBBackend) EditRecord(id int, update ModifyFun) (bool, string) {
	needsInsert := false
	rec := d.FindById(id)
	if rec == nil {
		needsInsert = true
		rec = &Component{Id: id}
	}
	before := *rec
	if update(rec) {
		if rec.Id != id {
			return false, "ID was modified."
		}
		// We're not in the business in modifying this.
		rec.Equiv_set = before.Equiv_set

		if *rec == before {
			return false, "No change."
		}
		var err error

		var toExec *sql.Stmt
		if needsInsert {
			toExec = d.insertRecord
		} else {
			toExec = d.updateRecord
		}
		result, err := toExec.Exec(id, time.Now(),
			nullIfEmpty(rec.Category), nullIfEmpty(rec.Value),
			nullIfEmpty(rec.Description), nullIfEmpty(rec.Notes),
			nullIfEmpty(rec.Quantity), nullIfEmpty(rec.Datasheet_url),
			rec.Drawersize, rec.Footprint)

		if err != nil {
			log.Printf("Oops: %s", err)
			return false, err.Error()
		}
		affected, _ := result.RowsAffected()
		if affected != 1 {
			log.Printf("Oops, expected 1 row to update but was %d", affected)
			return false, "ERR: not updated"
		}
		d.fts.Update(rec)

		json, _ := json.Marshal(rec)
		log.Printf("STORE %s", json)

		return true, ""
	}
	return false, ""
}

func (d *DBBackend) JoinSet(id int, set int) {
	d.LeaveSet(id) // precondition.
	d.joinSet.Exec(id, set)
}

func (d *DBBackend) LeaveSet(id int) {
	// The limited way SQLite works, we have to find the equivalence
	// set first before we can update. Not really efficient, and we
	// would need a transaction here, but, yeah, good enough for a
	// 0.001 qps service :)
	c := d.FindById(id)
	if c != nil {
		d.leaveSet.Exec(id, c.Equiv_set)
	}
}

func (d *DBBackend) MatchingEquivSetForComponent(id int) []*Component {
	result := make([]*Component, 0, 10)
	rows, _ := d.findEquivById.Query(id)
	for rows != nil && rows.Next() {
		c, _ := row2Component(rows)
		result = append(result, c)
	}
	rows.Close()
	return result
}

func (d *DBBackend) Search(search_term string) *SearchResult {
	return d.fts.Search(search_term)
}
