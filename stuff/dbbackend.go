package main

import (
	"database/sql"
	_ "github.com/lib/pq"
	"log"
	"time"
)

type DBBackend struct {
	db           *sql.DB
	findById     *sql.Stmt
	insertRecord *sql.Stmt
	updateRecord *sql.Stmt
}

func NewDBBackend(db *sql.DB) (*DBBackend, error) {
	findById, err := db.Prepare("SELECT category, value, description, notes, quantity, datasheet_url,drawersize" +
		" FROM component where id=$1")
	if err != nil {
		return nil, err
	}
	insertRecord, err := db.Prepare("INSERT INTO component (id, created, updated, category, value, description, notes, quantity, datasheet_url,drawersize) " +
		" VALUES ($1, $2, $2, $3, $4, $5, $6, $7, $8, $9)")
	if err != nil {
		return nil, err
	}
	updateRecord, err := db.Prepare("UPDATE component SET " +
		"updated=$2, category=$3, value=$4, description=$5, notes=$6, quantity=$7, datasheet_url=$8,drawersize=$9 where id=$1 ")
	if err != nil {
		return nil, err
	}

	return &DBBackend{
		db:           db,
		findById:     findById,
		insertRecord: insertRecord,
		updateRecord: updateRecord}, nil
}

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

func (d *DBBackend) FindById(id int) *Component {
	type ReadRecord struct {
		category    *string
		value       *string
		description *string
		notes       *string
		quantity    *string
		datasheet   *string
		drawersize  *int
	}
	rec := &ReadRecord{}
	err := d.findById.QueryRow(id).Scan(&rec.category, &rec.value,
		&rec.description, &rec.notes, &rec.quantity, &rec.datasheet, &rec.drawersize)
	drawersize := 0
	if rec.drawersize != nil {
		drawersize = *rec.drawersize
	}
	switch {
	case err == sql.ErrNoRows:
		return nil
	case err != nil:
		log.Fatal(err)
	default:
		result := &Component{
			Id:            id,
			Category:      emptyIfNull(rec.category),
			Value:         emptyIfNull(rec.value),
			Description:   emptyIfNull(rec.description),
			Notes:         emptyIfNull(rec.notes),
			Quantity:      emptyIfNull(rec.quantity),
			Datasheet_url: emptyIfNull(rec.datasheet),
			Drawersize:    drawersize,
		}
		return result
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

		if needsInsert {
			_, err = d.insertRecord.Exec(id, time.Now(),
				nullIfEmpty(rec.Category), nullIfEmpty(rec.Value),
				nullIfEmpty(rec.Description), nullIfEmpty(rec.Notes),
				nullIfEmpty(rec.Quantity), nullIfEmpty(rec.Datasheet_url),
				rec.Drawersize)
		} else {
			_, err = d.updateRecord.Exec(id, time.Now(),
				nullIfEmpty(rec.Category), nullIfEmpty(rec.Value),
				nullIfEmpty(rec.Description), nullIfEmpty(rec.Notes),
				nullIfEmpty(rec.Quantity), nullIfEmpty(rec.Datasheet_url),
				rec.Drawersize)
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
