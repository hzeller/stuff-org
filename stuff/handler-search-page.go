// Handling search: show page, deal with JSON requests.
// Also: provide a more clean API.
package main

import (
	"encoding/json"
	"fmt"
	"html"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func showSearchPage(out http.ResponseWriter, r *http.Request) {
	out.Header().Set("Content-Type", "text/html; charset=utf-8")
	content, _ := ioutil.ReadFile("template/search-result.html")
	out.Write(content)
}

type JsonComponent struct {
	Component
	Image string `json:"img"`
}
type JsonApiSearchResult struct {
	Directlink string          `json:"link"`
	Items      []JsonComponent `json:"components"`
}

func encodeUriComponent(str string) string {
	u, err := url.Parse(str)
	if err != nil {
		return ""
	}
	return u.String()
}
func apiSearch(store StuffStore, out http.ResponseWriter, r *http.Request) {
	// Allow very brief caching, so that editing the query does not
	// necessarily has to trigger a new server roundtrip.
	out.Header().Set("Cache-Control", "max-age=10")
	out.Header().Set("Content-Type", "application/json")
	defaultOutLen := 20
	maxOutLen := 100 // Limit max output
	query := r.FormValue("q")
	limit, _ := strconv.Atoi(r.FormValue("count"))
	if limit <= 0 {
		limit = defaultOutLen
	}
	if limit > maxOutLen {
		limit = maxOutLen
	}
	var searchResults []*Component
	if query != "" {
		searchResults = store.Search(query)
	}
	outlen := limit
	if len(searchResults) < limit {
		outlen = len(searchResults)
	}
	jsonResult := &JsonApiSearchResult{
		Directlink: encodeUriComponent("/search#" + query),
		Items:      make([]JsonComponent, outlen),
	}

	for i := 0; i < outlen; i++ {
		var c = searchResults[i]
		jsonResult.Items[i].Component = *c
		jsonResult.Items[i].Image = fmt.Sprintf("/img/%d", c.Id)
	}

	json, _ := json.Marshal(jsonResult)
	out.Write(json)
}

// Pre-formatted search for quick div replacements.
type JsonHtmlSearchResultRecord struct {
	Id    int    `json:"id"`
	Label string `json:"txt"`
}

type JsonHtmlSearchResult struct {
	Count int                          `json:"count"`
	Info  string                       `json:"info"`
	Items []JsonHtmlSearchResultRecord `json:"items"`
}

func apiSearchPageItem(store StuffStore, out http.ResponseWriter, r *http.Request) {
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
	jsonResult := &JsonHtmlSearchResult{
		Count: len(searchResults),
		Info:  fmt.Sprintf("%d results (%s)", len(searchResults), elapsed),
		Items: make([]JsonHtmlSearchResultRecord, outlen),
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
