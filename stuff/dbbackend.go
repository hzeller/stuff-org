package main

import (
	"database/sql"
	"log"
	"time"
)

// Initial phase: while collecting the raw information, a single flat table
// is sufficient.
var create_schema string = `
create table component (
       id            int           constraint pk_component primary key,
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
       updated timestamp

       -- also, we need the following eventually
       -- labeltext, drawer-type, location. Several of these should have foreign keys.
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
		&rec.drawersize, &rec.footprint)
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
	db           *sql.DB
	findById     *sql.Stmt
	insertRecord *sql.Stmt
	updateRecord *sql.Stmt
	fts          *FulltextSearch
}

func NewDBBackend(db *sql.DB, create_tables bool) (*DBBackend, error) {
	if create_tables {
		_, err := db.Exec(create_schema)
		if err != nil {
			log.Fatal(err)
		}
	}
	all_fields := "category, value, description, notes, quantity, datasheet_url,drawersize,footprint"
	findById, err := db.Prepare("SELECT id, " + all_fields + " FROM component where id=$1")
	if err != nil {
		return nil, err
	}
	insertRecord, err := db.Prepare("INSERT INTO component (id, created, updated, " + all_fields + ") " +
		" VALUES ($1, $2, $2, $3, $4, $5, $6, $7, $8, $9, $10)")
	if err != nil {
		return nil, err
	}
	updateRecord, err := db.Prepare("UPDATE component SET " +
		"updated=$1, category=$2, value=$3, description=$4, notes=$5, quantity=$6, datasheet_url=$7, drawersize=$8, footprint=$9 WHERE id=$10")
	if err != nil {
		return nil, err
	}

	// Populate fts with existing components.
	fts := NewFulltextSearch()
	rows, _ := db.Query("SELECT id, " + all_fields + " FROM component")
	count := 0
	for rows != nil && rows.Next() {
		c, _ := row2Component(rows)
		fts.Update(c)
		count++
	}
	log.Printf("Prepopulated full text search with %d items", count)
	return &DBBackend{
		db:           db,
		findById:     findById,
		insertRecord: insertRecord,
		updateRecord: updateRecord,
		fts:          fts}, nil
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
		if *rec == before {
			return false, "No change."
		}
		var err error

		//var toExec *sql.Stmt
		var r sql.Result
		if needsInsert {
			r, err = d.insertRecord.Exec(id, time.Now(),
				nullIfEmpty(rec.Category), nullIfEmpty(rec.Value),
				nullIfEmpty(rec.Description), nullIfEmpty(rec.Notes),
				nullIfEmpty(rec.Quantity), nullIfEmpty(rec.Datasheet_url),
				rec.Drawersize, rec.Footprint)

		} else {
			r, err = d.updateRecord.Exec(time.Now(),
				nullIfEmpty(rec.Category), nullIfEmpty(rec.Value),
				nullIfEmpty(rec.Description), nullIfEmpty(rec.Notes),
				nullIfEmpty(rec.Quantity), nullIfEmpty(rec.Datasheet_url),
				rec.Drawersize, rec.Footprint, id)
		}

		if err != nil {
			log.Printf("Oops: %s", err)
			return false, err.Error()
		}
		affected, _ := r.RowsAffected()
		if affected != 1 {
			log.Printf("Oops, unexpected row number %d", affected)
			return false, "ERR: not updated"
		}
		d.fts.Update(rec)
		return true, ""
	}
	return false, ""
}

func (d *DBBackend) Search(search_term string) []*Component {
	return d.fts.Search(search_term)
}
