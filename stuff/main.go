// stuff store. Backed by a database.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/mattn/go-sqlite3"
)

// SearchResult holds metadata about the search.
type SearchResult struct {
	OrignialQuery  string
	RewrittenQuery string
	Results        []*Component
}

var wantTimings = flag.Bool("want-timings", false, "Print processing timings.")

func ElapsedPrint(msg string, start time.Time) {
	if *wantTimings {
		log.Printf("%s took %s", msg, time.Since(start))
	}
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
	bindAddress := flag.String("bind-address", ":2000", "Listen address:port to serve from")
	dbFile := flag.String("dbfile", "stuff-database.db", "SQLite database file")
	logfile := flag.String("logfile", "", "Logfile to write interesting events")
	do_cleanup := flag.Bool("cleanup-db", false, "Cleanup run of database")
	permitted_nets := flag.String("edit-permission-nets", "", "Comma separated list of networks (CIDR format IP-Addr/network) that are allowed to edit content")
	site_name := flag.String("site-name", "", "Site-name, in particular needed for SSL")
	ssl_key := flag.String("ssl-key", "", "Key file")
	ssl_cert := flag.String("ssl-cert", "", "Cert file")

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
	store, err = NewSqlStuffStore(db, is_dbfilenew)
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
	imagehandler := AddImageHandler(store, templates, *imageDir, *staticResource)
	AddFormHandler(store, templates, *imageDir, edit_nets)
	AddSearchHandler(store, templates, imagehandler)
	AddStatusHandler(store, templates, *imageDir)
	AddSitemapHandler(store, *site_name)
	http.Handle("/metrics", promhttp.Handler())

	log.Printf("Listening on %q", *bindAddress)
	if *ssl_cert != "" && *ssl_key != "" {
		log.Fatal(http.ListenAndServeTLS(*bindAddress,
			*ssl_cert, *ssl_key,
			nil))
	} else {
		log.Fatal(http.ListenAndServe(*bindAddress, nil))
	}

	var block_forever chan bool
	<-block_forever
}
