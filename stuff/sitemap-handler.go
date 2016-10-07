package main

import (
	"fmt"
	"net/http"
)

const (
	kSitemap = "/sitemap.txt"
)

type SitemapHandler struct {
	store      StuffStore
	siteprefix string
}

func AddSitemapHandler(store StuffStore, siteprefix string) {
	handler := &SitemapHandler{
		store:      store,
		siteprefix: siteprefix,
	}
	http.Handle(kSitemap, handler)
}

func (h *SitemapHandler) ServeHTTP(out http.ResponseWriter, req *http.Request) {
	out.Header().Set("Content-Type", "text/plain; charset=utf-8")
	h.store.IterateAll(func(c *Component) bool {
		fmt.Fprintf(out, "%s/form?id=%d\n", h.siteprefix, c.Id)
		return true
	})
}
