package main

import (
	"compress/gzip"
	"fmt"
	"math"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// First, very crude version of radio button selecions
type Selection struct {
	Value        string
	IsSelected   bool
	AddSeparator bool
}
type FormPage struct {
	Component     // All these values are shown in the form
	Version   int // Caching-relevant versioning of images.

	// Category choice.
	CatChoice    []Selection
	CatFallback  Selection
	CategoryText string

	// Status around current item; link to relevant group.
	HundredGroup int
	Status       []StatusItem

	// Additional stuff
	Msg    string // Feedback for user
	PrevId int    // For browsing
	NextId int

	FormEditable   bool
	ShowEditToggle bool
}

// -- TODO: For cleanup, we need some kind of category-aware plugin structure.

func cleanupResistor(component *Component) {
	optional_ppm, _ := regexp.Compile(`(?i)[,;]\s*(\d+\s*ppm)`)
	if match := optional_ppm.FindStringSubmatch(component.Value); match != nil {
		component.Description = strings.ToLower(match[1]) + "; " + component.Description
		component.Value = optional_ppm.ReplaceAllString(component.Value, "")
	}

	// Move percent into description.
	optional_percent, _ := regexp.Compile(`[,;]\s*((\+/-\s*)?(0?\.)?\d+\%)`)
	if match := optional_percent.FindStringSubmatch(component.Value); match != nil {
		component.Description = match[1] + "; " + component.Description
		component.Value = optional_percent.ReplaceAllString(component.Value, "")
	}

	optional_watt, _ := regexp.Compile(`(?i)[,;]\s*((((\d*\.)?\d+)|(\d+/\d+))\s*W(att)?)`)
	if match := optional_watt.FindStringSubmatch(component.Value); match != nil {
		component.Description = match[1] + "; " + component.Description
		component.Value = optional_watt.ReplaceAllString(component.Value, "")
	}

	// Get rid of Ohm
	optional_ohm, _ := regexp.Compile(`(?i)\s*ohm`)
	component.Value = optional_ohm.ReplaceAllString(component.Value, "")

	// Upper-case kilo at end or with spaces before are replaced
	// with simple 'k'.
	spaced_upper_kilo, _ := regexp.Compile(`(?i)\s*k$`)
	component.Value = spaced_upper_kilo.ReplaceAllString(component.Value, "k")

	component.Description = cleanString(component.Description)
	component.Value = cleanString(component.Value)
}

func cleanupFootprint(component *Component) {
	component.Footprint = cleanString(component.Footprint)

	to_package, _ := regexp.Compile(`(?i)^to-?(\d+)`)
	component.Footprint = to_package.ReplaceAllString(component.Footprint, "TO-$1")

	// For sip/dip packages: canonicalize to _p_ and end and move digits to end.
	sdip_package, _ := regexp.Compile(`(?i)^((\d+)[ -]?)?([sd])i[lp][ -]?(\d+)?`)
	if match := sdip_package.FindStringSubmatch(component.Footprint); match != nil {
		component.Footprint = sdip_package.ReplaceAllStringFunc(component.Footprint,
			func(string) string {
				return strings.ToUpper(match[3] + "IP-" + match[2] + match[4])
			})
	}
}

// Format a float value with single digit precision, but remove unneccessary .0
// (mmh, this looks like there should be some standard formatting modifier that
// does exactly that.
func fmtFloatNoZero(f float64) string {
	result := fmt.Sprintf("%.1f", f)
	if strings.HasSuffix(result, ".0") {
		return result[:len(result)-2] // remove trailing zero
	} else {
		return result
	}
}

func makeCapacitanceString(farad float64) string {
	switch {
	case farad < 1000e-12:
		return fmtFloatNoZero(farad*1e12) + "pF"
	case farad < 1000e-9:
		return fmtFloatNoZero(farad*1e9) + "nF"
	default:
		return fmtFloatNoZero(farad*1e6) + "uF"
	}
}

func translateCapacitorToleranceLetter(letter string) string {
	switch strings.ToLower(letter) {
	case "d":
		return "+/- 0.5pF"
	case "f":
		return "+/- 1%"
	case "g":
		return "+/- 2%"
	case "h":
		return "+/- 3%"
	case "j":
		return "+/- 5%"
	case "k":
		return "+/- 10%"
	case "m":
		return "+/- 20%"
	case "p":
		return "+100%,-0%"
	case "z":
		return "+80%,-20%"
	}
	return ""
}

func cleanupCapacitor(component *Component) {
	farad_value, _ := regexp.Compile(`(?i)^((\d*.)?\d+)\s*([uµnp])F(.*)$`)
	three_digit, _ := regexp.Compile(`(?i)^(\d\d)(\d)\s*([dfghjkmpz])?$`)
	if match := farad_value.FindStringSubmatch(component.Value); match != nil {
		number_digits := match[1]
		factor_character := match[3]
		trailing := cleanString(match[4])
		// Sometimes, values are written as strange multiples, e.g. 100nF is
		// sometimes written as 0.1uF. Normalize here.
		factor := 1e-6
		switch factor_character {
		case "u", "U", "µ":
			factor = 1e-6
		case "n", "N":
			factor = 1e-9
		case "p", "P":
			factor = 1e-12
		}
		val, err := strconv.ParseFloat(number_digits, 32)
		if err != nil || val < 0 {
			return // Strange value. Don't touch.
		}
		component.Value = makeCapacitanceString(val * factor)
		if len(trailing) > 0 {
			if len(component.Description) > 0 {
				component.Description = trailing + "; " + component.Description
			} else {
				component.Description = trailing
			}
		}
	} else if match := three_digit.FindStringSubmatch(component.Value); match != nil {
		value, _ := strconv.ParseFloat(match[1], 32)
		magnitude, _ := strconv.ParseFloat(match[2], 32)
		tolerance_letter := match[3]
		if magnitude < 0 || magnitude > 6 {
			return
		}
		multiplier := math.Exp(magnitude*math.Log(10)) * 1e-12
		component.Value = makeCapacitanceString(value * multiplier)
		tolerance := translateCapacitorToleranceLetter(tolerance_letter)
		if len(tolerance) > 0 {
			if len(component.Description) > 0 {
				component.Description = tolerance + "; " + component.Description
			} else {
				component.Description = tolerance
			}
		}
	}
}

func cleanString(input string) string {
	result := strings.TrimSpace(input)
	return strings.Replace(result, "\r\n", "\n", -1)
}

func cleanupCompoent(component *Component) {
	component.Value = cleanString(component.Value)
	component.Category = cleanString(component.Category)
	component.Description = cleanString(component.Description)
	component.Quantity = cleanString(component.Quantity)
	component.Notes = cleanString(component.Notes)
	component.Datasheet_url = cleanString(component.Datasheet_url)
	cleanupFootprint(component)

	// We should have pluggable cleanup modules per category. For
	// now just a quick hack.
	switch component.Category {
	case "Resistor":
		cleanupResistor(component)
	case "Capacitor (C)", "Aluminum Cap":
		cleanupCapacitor(component)
	}
}

// If this particular request is allowed to edit. Can depend on IP address,
// cookies etc.
func EditAllowed(r *http.Request, allowed_nets []*net.IPNet) bool {
	if allowed_nets == nil || len(allowed_nets) == 0 {
		return true // No restrictions.
	}
	// Looks like we can't get the IP address in its raw form from
	// the request, but have to parse it.
	addr := r.RemoteAddr
	if pos := strings.LastIndex(addr, ":"); pos >= 0 {
		addr = addr[0:pos]
	}
	var ip net.IP
	if ip = net.ParseIP(addr); ip == nil {
		return false
	}
	for i := 0; i < len(allowed_nets); i++ {
		if allowed_nets[i].Contains(ip) {
			return true
		}
	}
	return false
}

// TODO: all these parameters such as imageDir and edit_neds suggest that we
// want a struct here.
func entryFormHandler(store StuffStore, imageDir string, edit_nets []*net.IPNet,
	w http.ResponseWriter, r *http.Request) {

	// Look at the request and see what we need to display,
	// and if we have to store something.

	// Store-ID is a hidden field and only set if the form
	// is submitted.
	edit_id, _ := strconv.Atoi(r.FormValue("edit_id"))
	var next_id int
	if r.FormValue("nav_id_button") != "" {
		// Use the navigation buttons to choose next ID.
		next_id, _ = strconv.Atoi(r.FormValue("nav_id_button"))
	} else if r.FormValue("id") != r.FormValue("edit_id") {
		// The ID field was edited. Use that as the next ID the
		// user wants to jump to.
		next_id, _ = strconv.Atoi(r.FormValue("id"))
	} else if edit_id > 0 {
		// Regular submit. Jump to next
		next_id = edit_id + 1
	} else if cookie, err := r.Cookie("last-edit"); err == nil {
		// Last straw: what we remember from last edit.
		next_id, _ = strconv.Atoi(cookie.Value)
	}

	requestStore := r.FormValue("edit_id") != ""
	msg := ""
	page := &FormPage{
		// Version number is some value to be added to the image
		// URL so that the browser is forced to fetch it, independent of
		// its cache. This could be the last updated timestamp,
		// but for now, we just give it a sufficiently random number.
		Version: int(time.Now().UnixNano() % 10000),
	}

	edit_allowed := EditAllowed(r, edit_nets)

	defer ElapsedPrint("Form action", time.Now())

	if requestStore && edit_allowed {
		drawersize, _ := strconv.Atoi(r.FormValue("drawersize"))
		fromForm := Component{
			Id:            edit_id,
			Value:         r.FormValue("value"),
			Description:   r.FormValue("description"),
			Notes:         r.FormValue("notes"),
			Quantity:      r.FormValue("quantity"),
			Datasheet_url: r.FormValue("datasheet"),
			Drawersize:    drawersize,
			Footprint:     r.FormValue("footprint"),
		}
		// If there only was a ?: operator ...
		if r.FormValue("category_select") == "-" {
			fromForm.Category = r.FormValue("category_txt")
		} else {
			fromForm.Category = r.FormValue("category_select")
		}

		cleanupCompoent(&fromForm)

		was_stored, store_msg := store.EditRecord(edit_id, func(comp *Component) bool {
			*comp = fromForm
			return true
		})
		if was_stored {
			msg = fmt.Sprintf("Stored item %d; Proceed to %d", edit_id, next_id)
		} else {
			msg = fmt.Sprintf("Item %d (%s); Proceed to %d", edit_id, store_msg, next_id)
		}
	} else {
		msg = "Browse item " + fmt.Sprintf("%d", next_id)
	}

	// -- Populate form relevant fields.
	id := next_id
	page.Id = id
	if id > 0 {
		page.PrevId = id - 1
	}
	page.NextId = id + 1
	page.HundredGroup = (id / 100) * 100

	page.ShowEditToggle = edit_allowed

	// If the last request was an edit (requestStore), then we are on
	// a roll and have the next page form editable as well.
	// If we were merely viewing the page, then next edit is view as well.
	page.FormEditable = requestStore && edit_allowed

	currentItem := store.FindById(id)
	if currentItem != nil {
		page.Component = *currentItem
	} else {
		msg = msg + fmt.Sprintf(" (%d: New item)", id)
	}

	page.CatChoice = make([]Selection, len(available_category))
	anySelected := false
	for i, val := range available_category {
		thisSelected := page.Component.Category == val
		anySelected = anySelected || thisSelected
		page.CatChoice[i] = Selection{
			Value:        val,
			IsSelected:   thisSelected,
			AddSeparator: i%3 == 0}
	}
	page.CatFallback = Selection{
		Value:      "-",
		IsSelected: !anySelected}
	if !anySelected {
		page.CategoryText = page.Component.Category
	}
	page.Msg = msg

	// -- Populate status of fields in current block of 10
	page.Status = make([]StatusItem, 12)
	startStatusId := (id/10)*10 - 1
	if startStatusId <= 0 {
		startStatusId = 0
	}
	for i := 0; i < 12; i++ {
		fillStatusItem(store, imageDir, i+startStatusId, &page.Status[i])
		if i+startStatusId == id {
			page.Status[i].Status = page.Status[i].Status + " selstatus"
		}
	}

	// -- Output
	// We are not using any web-framework or want to keep track of session
	// cookies. Simply a barebone, state-less web app: use plain cookies.
	w.Header().Set("Set-Cookie", fmt.Sprintf("last-edit=%d", id))
	w.Header().Set("Content-Encoding", "gzip")
	zipped := gzip.NewWriter(w)
	renderTemplate(zipped, w.Header(), "form-template.html", page)
	zipped.Close()
}

func relatedComponentSetOperations(store StuffStore, edit_nets []*net.IPNet,
	out http.ResponseWriter, r *http.Request) {
	switch r.FormValue("op") {
	case "html":
		relatedComponentSetHtml(store, out, r)
	case "join":
		relatedComponentSetJoin(store, edit_nets, out, r)
	case "remove":
		relatedComponentSetRemove(store, edit_nets, out, r)
	}
}

func relatedComponentSetJoin(store StuffStore, edit_nets []*net.IPNet,
	out http.ResponseWriter, r *http.Request) {
	comp, err := strconv.Atoi(r.FormValue("comp"))
	if err != nil {
		return
	}
	set, err := strconv.Atoi(r.FormValue("set"))
	if err != nil {
		return
	}
	if EditAllowed(r, edit_nets) {
		store.JoinSet(comp, set)
	}
	relatedComponentSetHtml(store, out, r)
}

func relatedComponentSetRemove(store StuffStore, edit_nets []*net.IPNet,
	out http.ResponseWriter, r *http.Request) {
	comp, err := strconv.Atoi(r.FormValue("comp"))
	if err != nil {
		return
	}
	if EditAllowed(r, edit_nets) {
		store.LeaveSet(comp)
	}
	relatedComponentSetHtml(store, out, r)
}

type EquivalenceSet struct {
	Id    int
	Items []*Component
}
type EquivalenceSetList struct {
	HighlightComp int
	Message       string
	Sets          []*EquivalenceSet
}

func relatedComponentSetHtml(store StuffStore,
	out http.ResponseWriter, r *http.Request) {
	comp_id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		return
	}
	page := &EquivalenceSetList{
		HighlightComp: comp_id,
		Sets:          make([]*EquivalenceSet, 0, 0),
	}
	var current_set *EquivalenceSet = nil
	components := store.MatchingEquivSetForComponent(comp_id)
	switch len(components) {
	case 0:
		page.Message = "No Value or Category set"
	case 1:
		page.Message = "Only one component with this Category/Name"
	default:
		page.Message = "Organize matching components into same virtual drawer (drag'n drop)"
	}

	for _, c := range components {
		if current_set != nil && c.Equiv_set != current_set.Id {
			current_set = nil
		}

		if current_set == nil {
			current_set = &EquivalenceSet{
				Id:    c.Equiv_set,
				Items: make([]*Component, 0, 5),
			}
			page.Sets = append(page.Sets, current_set)
		}
		current_set.Items = append(current_set.Items, c)
	}
	renderTemplate(out, out.Header(), "set-drag-drop.html", page)
}
