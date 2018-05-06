package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	kFormPage = "/form"
	kSetApi   = "/api/related-set"
)

// Some useful pre-defined set of categories
var available_category []string = []string{
	"Fabric Art", "Paper Art", "Drawing",
	"3D print", "Book", "Printer Supply",
	"Electronics", "Stepper Motor", "Tools",
}

type FormHandler struct {
	store    StuffStore
	template *TemplateRenderer
	imgPath  string
	editNets []*net.IPNet // IP Networks that are allowed to edit
}

func AddFormHandler(store StuffStore, template *TemplateRenderer, imgPath string, editNets []*net.IPNet) {
	handler := &FormHandler{
		store:    store,
		template: template,
		imgPath:  imgPath,
		editNets: editNets,
	}
	http.Handle(kFormPage, handler)
	http.Handle(kSetApi, handler)
}

func (h *FormHandler) ServeHTTP(out http.ResponseWriter, req *http.Request) {
	switch {
	case strings.HasPrefix(req.URL.Path, kSetApi):
		h.relatedComponentSetOperations(out, req)
	default:
		h.entryFormHandler(out, req)
	}
}

// First, very crude version of radio button selecions
type Selection struct {
	Value        string
	IsSelected   bool
	AddSeparator bool
}
type FormPage struct {
	Component         // All these values are shown in the form
	PageTitle         string
	ImageUrl          string
	DatasheetLinkText string // Abbreviated link for display

	DescriptionRows int // Number of rows displayed in textarea
	NotesRows       int

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

func cleanupResistor(c *Component) {
	optional_ppm, _ := regexp.Compile(`(?i)[,;]\s*(\d+\s*ppm)`)
	if match := optional_ppm.FindStringSubmatch(c.Value); match != nil {
		c.Description = strings.ToLower(match[1]) + "; " + c.Description
		c.Value = optional_ppm.ReplaceAllString(c.Value, "")
	}

	// Move percent into description.
	optional_percent, _ := regexp.Compile(`[,;]\s*((\+/-\s*)?(0?\.)?\d+\%)`)
	if match := optional_percent.FindStringSubmatch(c.Value); match != nil {
		c.Description = match[1] + "; " + c.Description
		c.Value = optional_percent.ReplaceAllString(c.Value, "")
	}

	optional_watt, _ := regexp.Compile(`(?i)[,;]\s*((((\d*\.)?\d+)|(\d+/\d+))\s*W(att)?)`)
	if match := optional_watt.FindStringSubmatch(c.Value); match != nil {
		c.Description = match[1] + "; " + c.Description
		c.Value = optional_watt.ReplaceAllString(c.Value, "")
	}

	// Get rid of Ohm
	optional_ohm, _ := regexp.Compile(`(?i)\s*ohm`)
	c.Value = optional_ohm.ReplaceAllString(c.Value, "")

	// Upper-case kilo at end or with spaces before are replaced
	// with simple 'k'.
	spaced_upper_kilo, _ := regexp.Compile(`(?i)\s*k$`)
	c.Value = spaced_upper_kilo.ReplaceAllString(c.Value, "k")

	c.Description = cleanString(c.Description)
	c.Value = cleanString(c.Value)
}

func cleanupFootprint(c *Component) {
	c.Footprint = cleanString(c.Footprint)

	to_package, _ := regexp.Compile(`(?i)^to-?(\d+)`)
	c.Footprint = to_package.ReplaceAllString(c.Footprint, "TO-$1")

	// For sip/dip packages: canonicalize to _p_ and end and move digits to end.
	sdip_package, _ := regexp.Compile(`(?i)^((\d+)[ -]?)?p?([sd])i[lp][ -]?(\d+)?`)
	if match := sdip_package.FindStringSubmatch(c.Footprint); match != nil {
		c.Footprint = sdip_package.ReplaceAllStringFunc(c.Footprint,
			func(string) string {
				return strings.ToUpper(match[3] + "IP-" + match[2] + match[4])
			})
	}
}

func createLinkTextFromUrl(u string) string {
	if len(u) < 30 {
		return u
	}
	shortenurl, _ := regexp.Compile("(.*://)([^/]+)/(.*)([/?&].*)$")
	return shortenurl.ReplaceAllString(u, "$2/…$4")
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
		// Sometimes, values are written as strange multiples,
		// e.g. 100nF is sometimes written as 0.1uF. Normalize here.
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

func cleanupComponent(component *Component) {
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
func (h *FormHandler) EditAllowed(r *http.Request) bool {
	if h.editNets == nil || len(h.editNets) == 0 {
		return true // No restrictions.
	}
	addr := ""
	if h := r.Header["X-Forwarded-For"]; h != nil {
		addr = h[0]
	} else {
		addr := r.RemoteAddr
		if pos := strings.LastIndex(addr, ":"); pos >= 0 {
			addr = addr[0:pos]
		}
	}
	var ip net.IP
	if ip = net.ParseIP(addr); ip == nil {
		return false
	}
	for i := 0; i < len(h.editNets); i++ {
		if h.editNets[i].Contains(ip) {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func (h *FormHandler) entryFormHandler(w http.ResponseWriter, r *http.Request) {
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
	edit_allowed := h.EditAllowed(r)

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

		cleanupComponent(&fromForm)

		was_stored, store_msg := h.store.EditRecord(edit_id, func(comp *Component) bool {
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
	page := &FormPage{}
	id := next_id
	page.Id = id
	page.ImageUrl = fmt.Sprintf("/img/%d", id)
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
	if page.FormEditable {
		// While we edit an element, we might want to force non-caching
		// of the particular image by addinga semi-random number to it.
		// Why ? When we edit an element, we might just have updated the
		// image while the browser stubbonly cached the old version.
		page.ImageUrl += fmt.Sprintf("?version=%d",
			int(time.Now().UnixNano()%10000))
	}
	currentItem := h.store.FindById(id)
	http_code := http.StatusOK
	if currentItem != nil {
		page.Component = *currentItem
		if currentItem.Category != "" {
			page.PageTitle = currentItem.Category + " - "
		}
		page.PageTitle += currentItem.Value
		page.DatasheetLinkText = createLinkTextFromUrl(currentItem.Datasheet_url)
	} else {
		http_code = http.StatusNotFound
		msg = msg + fmt.Sprintf(" (%d: New item)", id)
		page.PageTitle = "New Item: CkP stuff organization"
	}

	page.DescriptionRows = max(3, strings.Count(page.Component.Description, "\n")+1)
	page.NotesRows = max(3, strings.Count(page.Component.Notes, "\n")+1)

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
		fillStatusItem(h.store, h.imgPath, i+startStatusId, &page.Status[i])
		if i+startStatusId == id {
			page.Status[i].Status = page.Status[i].Status + " selstatus"
		}
	}

	// -- Output
	// We are not using any web-framework or want to keep track of session
	// cookies. Simply a barebone, state-less web app: use plain cookies.
	w.Header().Set("Set-Cookie", fmt.Sprintf("last-edit=%d", id))
	var zipped io.WriteCloser = nil
	for _, val := range r.Header["Accept-Encoding"] {
		if strings.Contains(strings.ToLower(val), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			zipped = gzip.NewWriter(w)
			break
		}
	}

	if edit_allowed {
		h.template.RenderWithHttpCode(w, zipped, http_code,
			"form-template.html", page)
	} else {
		h.template.RenderWithHttpCode(w, zipped, http_code,
			"display-template.html", page)
	}
	if zipped != nil {
		zipped.Close()
	}
}

func (h *FormHandler) relatedComponentSetOperations(out http.ResponseWriter, r *http.Request) {
	switch r.FormValue("op") {
	case "html":
		h.relatedComponentSetHtml(out, r)
	case "join":
		h.relatedComponentSetJoin(out, r)
	case "remove":
		h.relatedComponentSetRemove(out, r)
	}
}

func (h *FormHandler) relatedComponentSetJoin(out http.ResponseWriter, r *http.Request) {
	comp, err := strconv.Atoi(r.FormValue("comp"))
	if err != nil {
		return
	}
	set, err := strconv.Atoi(r.FormValue("set"))
	if err != nil {
		return
	}
	if h.EditAllowed(r) {
		h.store.JoinSet(comp, set)
	}
	h.relatedComponentSetHtml(out, r)
}

func (h *FormHandler) relatedComponentSetRemove(out http.ResponseWriter, r *http.Request) {
	comp, err := strconv.Atoi(r.FormValue("comp"))
	if err != nil {
		return
	}
	if h.EditAllowed(r) {
		h.store.LeaveSet(comp)
	}
	h.relatedComponentSetHtml(out, r)
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

func (h *FormHandler) relatedComponentSetHtml(out http.ResponseWriter, r *http.Request) {
	comp_id, err := strconv.Atoi(r.FormValue("id"))
	if err != nil {
		return
	}
	page := &EquivalenceSetList{
		HighlightComp: comp_id,
		Sets:          make([]*EquivalenceSet, 0, 0),
	}
	var current_set *EquivalenceSet = nil
	components := h.store.MatchingEquivSetForComponent(comp_id)
	switch len(components) {
	case 0:
	case 1:
		page.Message = ""
	default:
		page.Message = "Organize matching components into same virtual drawer (drag'n drop)"
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
	}
	h.template.Render(out, "set-drag-drop.html", page)
}
