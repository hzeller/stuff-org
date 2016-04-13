package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	kStaticResource = "/static/"
	kComponentImage = "/img/"
)

type ImageHandler struct {
	store      StuffStore
	imgPath    string
	staticPath string
}

func AddImageHandler(store StuffStore, imgPath string, staticPath string) {
	handler := &ImageHandler{
		store:      store,
		imgPath:    imgPath,
		staticPath: staticPath,
	}
	http.Handle(kComponentImage, handler) // Serve an component image or fallback.
	http.Handle(kStaticResource, handler) // serve a static resource
}

func (h *ImageHandler) ServeHTTP(out http.ResponseWriter, req *http.Request) {
	switch {
	case strings.HasPrefix(req.URL.Path, kComponentImage):
		prefix_len := len(kComponentImage)
		requested := req.URL.Path[prefix_len:]
		h.serveComponentImage(requested, out, req)
	default:
		h.serveStatic(out, req)
	}
}

// Create a synthetic representation of component from information given
// in the component.
func serveGeneratedComponentImage(component *Component, category string, value string,
	out http.ResponseWriter) bool {
	// If we got a category string, it takes precedence
	if len(category) == 0 && component != nil {
		category = component.Category
	}
	switch category {
	case "Resistor":
		return serveResistorImage(component, value, out)
	case "Diode (D)":
		return renderTemplate(out, out.Header(), "category-Diode.svg", component)
	case "LED":
		return renderTemplate(out, out.Header(), "category-LED.svg", component)
	case "Capacitor (C)":
		return renderTemplate(out, out.Header(), "category-Capacitor.svg", component)
	}
	return false
}

func servePackageImage(component *Component, out http.ResponseWriter) bool {
	if component == nil || component.Footprint == "" {
		return false
	}
	return renderTemplate(out, out.Header(),
		"package-"+component.Footprint+".svg", component)
}

func (h *ImageHandler) serveComponentImage(requested string, out http.ResponseWriter, r *http.Request) {
	path := h.imgPath + "/" + requested + ".jpg"
	if _, err := os.Stat(path); err == nil { // we have an image.
		sendResource(path, h.staticPath+"/fallback.jpg", out)
		return
	}
	// No image, but let's see if we can do something from the
	// component
	if comp_id, err := strconv.Atoi(requested); err == nil {
		component := h.store.FindById(comp_id)
		category := r.FormValue("c") // We also allow these if available
		value := r.FormValue("v")
		if (component != nil || len(category) > 0 || len(value) > 0) &&
			serveGeneratedComponentImage(component, category, value, out) {
			return
		}
		if servePackageImage(component, out) {
			return
		}
	}
	// Use fallback-resource straight away to get short cache times.
	sendResource("", h.staticPath+"/fallback.jpg", out)
}

func (h *ImageHandler) serveStatic(out http.ResponseWriter, r *http.Request) {
	prefix_len := len("/static/")
	resource := r.URL.Path[prefix_len:]
	sendResource(h.staticPath+"/"+resource, "", out)
}

func sendResource(local_path string, fallback_resource string, out http.ResponseWriter) {
	cache_time := 900
	header_addon := ""
	content, _ := ioutil.ReadFile(local_path)
	if content == nil && fallback_resource != "" {
		local_path = fallback_resource
		content, _ = ioutil.ReadFile(local_path)
		cache_time = 10 // fallbacks might change more often.
		header_addon = ",must-revalidate"
	}
	out.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d%s", cache_time, header_addon))
	switch {
	case strings.HasSuffix(local_path, ".png"):
		out.Header().Set("Content-Type", "image/png")
	case strings.HasSuffix(local_path, ".css"):
		out.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(local_path, ".svg"):
		out.Header().Set("Content-Type", "image/svg+xml")
	default:
		out.Header().Set("Content-Type", "image/jpg")
	}

	out.Write(content)
}
