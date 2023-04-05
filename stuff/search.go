// A search that keeps everything in memory and knows about meaning
// of some component fields as well as how to create some nice scoring.
package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var (
	andRewrite              = regexp.MustCompile(`(?i)( and )`)
	orRewrite               = regexp.MustCompile(`(?i)( or )`)
	possibleResistor        = regexp.MustCompile(`(?i)([0-9]+(\.[0-9]+)*[kM]?)(\s*Ohm?)`)
	possibleSmallMicrofarad = regexp.MustCompile(`(?i)(0?\.[0-9]+)u(\w*)`)
	logicalTerm             = regexp.MustCompile(`(?i)([\(\)\|])`)
	likeTerm                = regexp.MustCompile(`(?i)like:([0-9]+)`)
)

// componentResolver converts a componentID to a string containing the
// component's terms or blank if the component doesn't exist.
type componentResolver func(componentID int) string

func isSeparator(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '.' || c == ',' || c == ';'
}

func queryRewrite(term string, componentLookup componentResolver) string {
	term = andRewrite.ReplaceAllString(term, " ")

	term = orRewrite.ReplaceAllString(term, " | ")

	term = possibleResistor.ReplaceAllString(term, "($0 | ($1 (resistor|potentiometer|r-network)))")

	// Nanofarad values are often given as 0.something microfarad.
	// Internally, all capacitors are normalized to nanofarad.
	if cmatch := possibleSmallMicrofarad.FindStringSubmatch(term); cmatch != nil {
		val, err := strconv.ParseFloat(cmatch[1], 32)
		if err == nil {
			term = possibleSmallMicrofarad.ReplaceAllString(
				term, fmt.Sprintf("($0 | %.0fn$2)", 1000*val))
		}
	}

	term = likeTerm.ReplaceAllStringFunc(term, func(match string) string {
		split := strings.SplitN(match, ":", 2)
		if len(split) != 2 {
			return match
		}
		val, err := strconv.ParseInt(split[1], 10, 32)
		if err != nil {
			return match
		}

		return fmt.Sprintf("(%s)", componentLookup(int(val)))
	})

	return term
}

func preprocessTerm(term string) string {
	// For simplistic parsing, add spaces around special characters (|)
	term = logicalTerm.ReplaceAllString(term, " $1 ")

	// * Lowercase: we want to be case insensitive
	// * Dash remove: we consider dashes to join words and we want to be
	//   agnostic to various spellings (might break down with minus signs
	//   (e.g. -50V), so might need refinement later)
	return strings.Replace(strings.ToLower(term), "-", "", -1)
}

func StringScore(needle string, haystack string) float32 {
	pos := strings.Index(haystack, needle)
	if pos < 0 {
		return 0
	}
	endword := pos + len(needle)
	var boost float32 = 0.0
	if pos == 0 || isSeparator(haystack[pos-1]) {
		boost = 12.0 // word starts with it
	}
	if endword == len(haystack) || isSeparator(haystack[endword]) {
		boost += 5.0 // word ends with it
	}
	result := 10 - pos // early in string: higher score
	if result < 1 {
		return 1 + boost
	} else {
		return float32(result) + boost
	}
}

func maxlist(values ...float32) (max float32) {
	max = 0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	return
}

// Score terms, starting at index 'start' and goes to the end of the current
// term (closing parenthesis or end of string). Returns score and
// last index it went up to.
// Treats consecutive terms as 'AND' until it reaches an 'OR' operator.
// Like in real life, precedence AND > OR, and there are parenthesis to eval
// terms differently.
//
// Scoring per component is done on a couple of important fields, but weighted
// according to their importance (e.g. the Value field scores more than Info).
//
// Since we are dealing with real number scores instead of simple boolean
// matches, the AND and OR operators are implemented to return results like
// that.
//   - If any of the subscore of an AND expression is zero, the result is zero.
//     Otherwise, all sub-scores are added up: this gives a meaningful ordering
//     for componets that match all terms in the AND expression.
//     the result is zero.
//   - For the OR-operation, we take the highest scoring sub-term. Thus if
//     multiple sub-terms in the OR expression match, this won't result in
//     keyword stuffing (though one could consider adding a much smaller
//     constant weight for number of sub-terms that do match).
func (c *SearchComponent) scoreTerms(terms []string, start int) (float32, int) {
	var last_or_term float32 = 0.0
	var current_score float32 = 0.0
	for i := start; i < len(terms); i++ {
		part := terms[i]
		if part == "(" && i < len(terms)-1 {
			sub_score, subterm_end := c.scoreTerms(terms, i+1)
			if sub_score <= 0 {
				current_score = -1000 // See below for reasoning
			} else {
				current_score += sub_score
			}
			i = subterm_end
			continue
		}
		if part == "|" {
			last_or_term = maxlist(last_or_term, current_score)
			current_score = 0
			continue
		}
		if part == ")" && start != 0 {
			return maxlist(last_or_term, current_score), i
		}
		// Avoid keyword stuffing by looking only at the field
		// that scores the most.
		// NOTE: more fields here, add to lowerCased below.
		score := maxlist(2.0*StringScore(part, c.preprocessed.Category),
			3.0*StringScore(part, c.preprocessed.Value),
			1.5*StringScore(part, c.preprocessed.Description),
			1.2*StringScore(part, c.preprocessed.Notes),
			1.0*StringScore(part, c.preprocessed.Footprint))
		if score == 0 {
			// We essentially would do an early out here, but
			// since we're in the middle of parsing until we reach
			// the next OR, we do the simplistic thing here:
			// just make it impossible to have max()
			// give a positive result with the last term.
			// (todo: if this becomes a problem, implement early out)
			current_score = -1000
		} else {
			current_score += score
		}
	}
	return maxlist(last_or_term, current_score), len(terms)
}

// Matches the component and returns a score
func (c *SearchComponent) MatchScore(term string) float32 {
	score, _ := c.scoreTerms(strings.Fields(term), 0)
	return score
}

// ToQuery converts the component into a normalized search query that can be
// used to find similar components.
func (c *SearchComponent) ToQuery() string {
	sb := &strings.Builder{}

	for _, tmp := range []string{
		c.preprocessed.Category,
		c.preprocessed.Description,
		c.preprocessed.Notes,
		c.preprocessed.Value,
		c.preprocessed.Footprint,
	} {
		sb.WriteString(tmp)
		sb.WriteString(" ")
	}

	return fmt.Sprintf("(%s)", strings.Join(strings.Fields(sb.String()), "|"))
}

type SearchComponent struct {
	orig         *Component
	preprocessed *Component
}
type FulltextSearch struct {
	lock         sync.RWMutex
	id2Component map[int]*SearchComponent
}

func NewFulltextSearch() *FulltextSearch {
	return &FulltextSearch{
		id2Component: make(map[int]*SearchComponent),
	}
}

type ScoredComponent struct {
	score float32
	comp  *Component
}
type ScoreList []*ScoredComponent

func (s ScoreList) Len() int {
	return len(s)
}
func (s ScoreList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s ScoreList) Less(a, b int) bool {
	diff := s[a].score - s[b].score
	if diff != 0 {
		// We want to reverse score: highest match first
		return diff > 0
	}

	if s[a].comp.Value != s[b].comp.Value {
		// Items that have a value vs. none are scored higher.
		if s[a].comp.Value == "" {
			return false
		}
		if s[b].comp.Value == "" {
			return true
		}
		// other than that: alphabetically
		return s[a].comp.Value < s[b].comp.Value
	}

	if s[a].comp.Description != s[b].comp.Description {
		// Items that have a Description vs. none are scored higher.
		if s[a].comp.Description == "" {
			return false
		}
		if s[b].comp.Description == "" {
			return true
		}
	}

	// If we reach this, make it at least predictable.
	return s[a].comp.Id < s[b].comp.Id // stable
}

func (s *FulltextSearch) Update(c *Component) {
	if c == nil {
		return
	}
	lowerCased := &Component{
		// Only the fields we are interested in.
		Category:    preprocessTerm(c.Category),
		Value:       preprocessTerm(c.Value),
		Description: preprocessTerm(c.Description),
		Notes:       preprocessTerm(c.Notes),
		Footprint:   preprocessTerm(c.Footprint),
	}
	s.lock.Lock()
	s.id2Component[c.Id] = &SearchComponent{
		orig:         c,
		preprocessed: lowerCased,
	}
	s.lock.Unlock()
}
func (s *FulltextSearch) Search(search_term string) *SearchResult {
	output := &SearchResult{
		OrignialQuery: search_term,
	}

	search_term = queryRewrite(search_term, s.componentTerms)
	output.RewrittenQuery = search_term
	search_term = preprocessTerm(search_term)
	s.lock.RLock()
	scoredlist := make(ScoreList, 0, 10)
	for _, search_comp := range s.id2Component {
		scored := &ScoredComponent{
			score: search_comp.MatchScore(search_term),
			comp:  search_comp.orig,
		}
		if scored.score > 0 {
			scoredlist = append(scoredlist, scored)
		}
	}
	s.lock.RUnlock()
	sort.Sort(ScoreList(scoredlist))
	output.Results = make([]*Component, len(scoredlist))
	for idx, scomp := range scoredlist {
		output.Results[idx] = scomp.comp
	}
	return output
}

func (s *FulltextSearch) componentTerms(componentID int) string {
	s.lock.RLock()
	defer s.lock.RUnlock()

	component, ok := s.id2Component[componentID]
	if !ok {
		return ""
	}

	return component.ToQuery()
}

// Validate that componentTerms is a componentResolver.
var _ (componentResolver) = ((*FulltextSearch)(nil)).componentTerms
