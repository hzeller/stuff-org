package main

import (
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
)

type TemplateRenderer struct {
	baseDir         string
	cachedTemplates *template.Template
	doCache         bool
}

func NewTemplateRenderer(baseDir string, doCache bool) *TemplateRenderer {
	result := &TemplateRenderer{
		baseDir: baseDir,
		doCache: doCache,
	}
	if doCache {
		// Ideally, we would like to register these later in the
		// handlers, but looks like we need to do that in one bunch.
		// So, TODO: wait until everything is registered, then
		// do the registration. Lazy for now.
		result.cachedTemplates = template.Must(template.ParseFiles(
			// Application templates
			baseDir+"/form-template.html",
			baseDir+"/status-table.html",
			baseDir+"/set-drag-drop.html",
			// Templates to create component images
			baseDir+"/component/category-Diode.svg",
			baseDir+"/component/category-LED.svg",
			baseDir+"/component/category-Capacitor.svg",
			// Value rendering of resistors
			baseDir+"/component/4-Band_Resistor.svg",
			baseDir+"/component/5-Band_Resistor.svg",
			// Some common packages
			baseDir+"/component/package-TO-39.svg",
			baseDir+"/component/package-TO-220.svg",
			baseDir+"/component/package-DIP-14.svg",
			baseDir+"/component/package-DIP-16.svg",
			baseDir+"/component/package-DIP-28.svg"))
	}
	return result
}

func setContentTypeFromTemplateName(template_name string, header http.Header) {
	switch {
	case strings.HasSuffix(template_name, ".svg"):
		header.Set("Content-Type", "image/svg+xml")
	default:
		header.Set("Content-Type", "text/html; charset=utf-8")
	}
}

// for now, render templates directly to easier edit them.
func (h *TemplateRenderer) Render(w io.Writer, header http.Header, template_name string, p interface{}) bool {
	var err error
	if h.doCache {
		templ := h.cachedTemplates.Lookup(template_name)
		if templ == nil {
			return false
		}
		setContentTypeFromTemplateName(template_name, header)
		err = templ.Execute(w, p)
	} else {
		t, err := template.ParseFiles(h.baseDir + "/" + template_name)
		if err != nil {
			t, err = template.ParseFiles(h.baseDir + "/component/" + template_name)
			if err != nil {
				log.Printf("%s: %s", template_name, err)
				return false
			}
		}
		setContentTypeFromTemplateName(template_name, header)
		err = t.Execute(w, p)
	}
	if err != nil {
		log.Printf("Template broken %s (%s)", template_name, err)
		return false
	}
	return true
}
