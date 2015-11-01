// Handling search: show page, deal with JSON requests.
package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"net/http"
	"time"
)

func showSearchPage(out http.ResponseWriter, r *http.Request) {
	out.Header().Set("Content-Type", "text/html; charset=utf-8")
	content, _ := ioutil.ReadFile("template/search-result.html")
	out.Write(content)
}

type JsonSearchResultRecord struct {
	Id    int    `json:"id"`
	Label string `json:"txt"`
}

type JsonSearchResult struct {
	Count int                      `json:"count"`
	Info  string                   `json:"info"`
	Items []JsonSearchResultRecord `json:"items"`
}

func apiSearch(store StuffStore, out http.ResponseWriter, r *http.Request) {
	defer ElapsedPrint("Query", time.Now())
	// Allow very brief caching, so that editing the query does not
	// necessarily has to trigger a new server roundtrip.
	out.Header().Set("Cache-Control", "max-age=10")
	query := r.FormValue("q")
	if query == "" {
		out.Write([]byte("{\"count\":0, \"info\":\"\", \"items\":[]}"))
		return
	}
	start := time.Now()
	searchResults := store.Search(query)
	elapsed := time.Now().Sub(start)
	elapsed = time.Microsecond * ((elapsed + time.Microsecond/2) / time.Microsecond)
	outlen := 24 // Limit max output
	if len(searchResults) < outlen {
		outlen = len(searchResults)
	}
	jsonResult := &JsonSearchResult{
		Count: len(searchResults),
		Info:  fmt.Sprintf("%d results (%s)", len(searchResults), elapsed),
		Items: make([]JsonSearchResultRecord, outlen),
	}

	for i := 0; i < outlen; i++ {
		var c = searchResults[i]
		jsonResult.Items[i].Id = c.Id
		jsonResult.Items[i].Label = "<b>" + html.EscapeString(c.Value) + "</b> " +
			html.EscapeString(c.Description) +
			fmt.Sprintf(" <span class='idtxt'>(ID:%d)</span>", c.Id)
	}
	json, _ := json.Marshal(jsonResult)
	out.Write(json)
}
