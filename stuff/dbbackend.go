package main

import (
	"database/sql"
	_ "github.com/lib/pq"
	"log"
	"time"
)

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
	fts          *FulltextSearh
}

func NewDBBackend(db *sql.DB) (*DBBackend, error) {
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
		"updated=$2, category=$3, value=$4, description=$5, notes=$6, quantity=$7, datasheet_url=$8,drawersize=$9, footprint=$10 where id=$1 ")
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
			return false, "ID was modified"
		}
		if *rec == before {
			log.Printf("No need to store ID=%d: no change.", id)
			return true, "No change"
		}
		var err error

		var toExec *sql.Stmt
		if needsInsert {
			toExec = d.insertRecord
		} else {
			toExec = d.updateRecord
		}
		_, err = toExec.Exec(id, time.Now(),
			nullIfEmpty(rec.Category), nullIfEmpty(rec.Value),
			nullIfEmpty(rec.Description), nullIfEmpty(rec.Notes),
			nullIfEmpty(rec.Quantity), nullIfEmpty(rec.Datasheet_url),
			rec.Drawersize, rec.Footprint)

		if err != nil {
			return false, err.Error()
		}
		d.fts.Update(rec)
	}
	return true, ""
}

func (d *DBBackend) Search(search_term string) []*Component {
	return d.fts.Search(search_term)
}
