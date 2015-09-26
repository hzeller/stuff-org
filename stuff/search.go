package main

import (
	"sort"
	"strings"
	"sync"
)

func isSeparator(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '.' || c == ',' || c == ';'
}

func StringScore(needle string, haystack string) float32 {
	pos := strings.Index(haystack, needle)
	if pos < 0 {
		return 0
	}
	endword := pos + len(needle)
	var boost float32 = 0.0
	if (endword == len(haystack) || isSeparator(haystack[endword])) &&
		(pos == 0 || isSeparator(haystack[pos-1])) {
		boost = 20.0 // exact word match: higher score.
	}
	result := 10 - pos // early in string: higher score
	if result < 1 {
		return 1 + boost
	} else {
		return float32(result) + boost
	}
}

// Matches the component and returns a score
func (c *Component) MatchScore(term string) float32 {
	var total_score float32 = 0.0
	for _, part := range strings.Split(term, " ") {
		score := 1.0*StringScore(part, strings.ToLower(c.Category)) +
			3.0*StringScore(part, strings.ToLower(c.Value)) +
			2.0*StringScore(part, strings.ToLower(c.Description))
		if score == 0 {
			return 0 // all words must match somehow
		} else {
			total_score += score
		}
	}
	return total_score
}

type FulltextSearh struct {
	lock         sync.Mutex
	id2Component map[int]*Component
}

func NewFulltextSearch() *FulltextSearh {
	return &FulltextSearh{
		id2Component: make(map[int]*Component),
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
func (s ScoreList) Less(i, j int) bool {
	// We want to reverse score: highest match first
	diff := s[i].score - s[j].score
	if diff != 0 {
		return diff > 0
	}
	if s[i].comp.Value != s[j].comp.Value {
		return s[i].comp.Value < s[j].comp.Value
	}
	return s[i].comp.Id < s[j].comp.Id // stable
}

func (s *FulltextSearh) Update(c *Component) {
	if c == nil {
		return
	}
	s.lock.Lock()
	s.id2Component[c.Id] = c
	s.lock.Unlock()
}
func (s *FulltextSearh) Search(search_term string) []*Component {
	search_term = strings.ToLower(search_term)
	s.lock.Lock()
	scoredlist := make(ScoreList, 0, 10)
	for _, comp := range s.id2Component {
		scored := &ScoredComponent{
			score: comp.MatchScore(search_term),
			comp:  comp,
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
