/*
 * TODO(hzeller): right now, this has pretty hard-coded assumptions of what
 * an component URL looks like (/img/<id>) and what the underlying image
 * is (h.imgPath + "/" + <id> + ".jpg"). This needs to change if we are to
 * provide multiple image per component. Maybe /img/<id> the main-image, and
 * /img/<id>/<subimage> ?
 */
package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

const (
	kStaticResource = "/static/"
	kComponentImage = "/img/"
)

type ImageHandler struct {
	store      StuffStore
	template   *TemplateRenderer
	imgPath    string
	staticPath string
}

// There can be multiple images per part. The main ID describes the
// part, the gallery-ID a number within that gallery.
type galleryImage struct {
	id         int // stuff-id
	gallery_id int // sub-id within a gallery
}

func AddImageHandler(store StuffStore, template *TemplateRenderer, imgPath string, staticPath string) *ImageHandler {
	handler := &ImageHandler{
		store:      store,
		template:   template,
		imgPath:    imgPath,
		staticPath: staticPath,
	}
	http.Handle(kComponentImage, handler) // Serve an component image or fallback.
	http.Handle(kStaticResource, handler) // serve a static resource

	// With serving robots.txt, image-handler should probably be named
	// static handler.
	http.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		sendResource(staticPath+"/robots.txt", "", w)
	})
	return handler
}

func (h *ImageHandler) ServeHTTP(out http.ResponseWriter, req *http.Request) {
	switch {
	case strings.HasPrefix(req.URL.Path, kComponentImage):
		prefix_len := len(kComponentImage)
		requested_str := req.URL.Path[prefix_len:]
		var requested galleryImage
		n, _ := fmt.Sscanf(requested_str, "%d/%d", &requested.id, &requested.gallery_id)
		if n >= 1 {
			h.serveComponentImage(requested, out, req)
		} else {
			sendResource(h.staticPath+"/fallback.png", "", out)
		}
	default:
		h.serveStatic(out, req)
	}
}

// Create a synthetic representation of component from information given
// in the component.
func (h *ImageHandler) serveGeneratedComponentImage(component *Component, category string, value string,
	out http.ResponseWriter) bool {
	// If we got a category string, it takes precedence
	if len(category) == 0 && component != nil {
		category = component.Category
	}
	switch category {
	case "Resistor":
		out.Header().Set("Cache-Control", "max-age=60")
		return serveResistorImage(component, value, h.template, out)
	case "Diode (D)":
		out.Header().Set("Cache-Control", "max-age=60")
		return h.template.Render(out, "category-Diode.svg", component)
	case "LED":
		out.Header().Set("Cache-Control", "max-age=60")
		return h.template.Render(out, "category-LED.svg", component)
	case "Capacitor (C)":
		out.Header().Set("Cache-Control", "max-age=60")
		return h.template.Render(out, "category-Capacitor.svg", component)
	}
	return false
}

func (h *ImageHandler) servePackageImage(component *Component, out http.ResponseWriter) bool {
	if component == nil || component.Footprint == "" {
		return false
	}
	return h.template.Render(out,
		"package-"+component.Footprint+".svg", component)
}

// Returns true if this component likely has an image. False, if we know
// for sure that it doesn't.
func (h *ImageHandler) hasComponentImage(component *Component) bool {
	if component == nil {
		return false
	}
	switch component.Category {
	case "Resistor", "Diode (D)", "LED", "Capacitor (C)":
		return true
	}
	_, err := os.Stat(fmt.Sprintf("%s/%d.jpg", h.imgPath, component.Id))
	return err == nil
}

func (h *ImageHandler) sendImageIfAvailable(path string, out http.ResponseWriter) bool {
	if _, err := os.Stat(path); err == nil {
		sendResource(path, h.staticPath+"/fallback.png", out)
		return true
	}
	return false
}

func (h *ImageHandler) serveComponentImage(requested galleryImage, out http.ResponseWriter, r *http.Request) {
	if h.sendImageIfAvailable(fmt.Sprintf("%s/%d/%d.jpg", h.imgPath, requested.id, requested.gallery_id), out) {
		return
	}
	if h.sendImageIfAvailable(fmt.Sprintf("%s/%d.jpg", h.imgPath, requested.id), out) {
		return
	}

	// No image, but let's see if we can do something from the
	// part ID itself.
	component := h.store.FindById(requested.id)
	category := r.FormValue("c") // We also allow these if available
	value := r.FormValue("v")
	if (component != nil || len(category) > 0 || len(value) > 0) &&
		h.serveGeneratedComponentImage(component, category, value, out) {
		return
	}
	if h.servePackageImage(component, out) {
		return
	}
}

func (h *ImageHandler) serveStatic(out http.ResponseWriter, r *http.Request) {
	prefix_len := len("/static/")
	resource := r.URL.Path[prefix_len:]
	sendResource(h.staticPath+"/"+resource, "", out)
}

func sendResource(local_path string, fallback_resource string, out http.ResponseWriter) {
	cache_time := 900
	header_addon := ""
	content, _ := os.ReadFile(local_path)
	if content == nil && fallback_resource != "" {
		local_path = fallback_resource
		content, _ = os.ReadFile(local_path)
		cache_time = 10 // fallbacks might change more often.
		out.WriteHeader(http.StatusNotFound)
		header_addon = ",must-revalidate"
	}
	out.Header().Set("Cache-Control", fmt.Sprintf("max-age=%d%s", cache_time, header_addon))
	switch {
	case strings.HasSuffix(local_path, ".png"):
		out.Header().Set("Content-Type", "image/png")
	case strings.HasSuffix(local_path, ".css"):
		out.Header().Set("Content-Type", "text/css")
	case strings.HasSuffix(local_path, ".svg"):
		out.Header().Set("Content-Type", "image/svg+xml;charset=utf-8")
	case strings.HasSuffix(local_path, ".txt"):
		out.Header().Set("Content-Type", "text/plain")
	case strings.HasSuffix(local_path, ".json"):
		out.Header().Set("Content-Type", "application/json")
	default:
		out.Header().Set("Content-Type", "image/jpg")
	}

	out.Write(content)
}
