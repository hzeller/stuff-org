package main

import (
	"sort"
	"strings"
	"sync"
)

func isSeparator(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '.' || c == ',' || c == ';'
}

func preprocessTerm(term string) string {
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

// Matches the component and returns a score
func (c *SearchComponent) MatchScore(term string) float32 {
	var total_score float32 = 0.0
	for _, part := range strings.Split(term, " ") {
		// Avoid keyword stuffing by looking only at the field
		// that scores the most.
		// NOTE: more fields here, add to lowerCased below.
		score := maxlist(2.0*StringScore(part, c.preprocessed.Category),
			3.0*StringScore(part, c.preprocessed.Value),
			1.5*StringScore(part, c.preprocessed.Description),
			1.2*StringScore(part, c.preprocessed.Notes),
			1.0*StringScore(part, c.preprocessed.Footprint))

		if score == 0 {
			return 0 // all words must match somehow
		} else {
			total_score += score
		}
	}
	return total_score
}

type SearchComponent struct {
	orig         *Component
	preprocessed *Component
}
type FulltextSearch struct {
	lock         sync.Mutex
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
func (s *FulltextSearch) Search(search_term string) []*Component {
	search_term = preprocessTerm(search_term)
	s.lock.Lock()
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
	s.lock.Unlock()
	sort.Sort(ScoreList(scoredlist))
	result := make([]*Component, len(scoredlist))
	for idx, scomp := range scoredlist {
		result[idx] = scomp.comp
	}
	return result
}
