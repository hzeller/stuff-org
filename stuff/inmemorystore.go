package main

import (
	"sort"
	"sync"
)

type InMemoryStore struct {
	lock         sync.Mutex
	id2Component map[int]*Component
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		id2Component: make(map[int]*Component),
	}
}
func (s *InMemoryStore) EditRecord(id int, update ModifyFun) (bool, string) {
	var toEdit Component
	s.lock.Lock()
	found := s.id2Component[id]
	if found != nil {
		toEdit = *found
	} else {
		toEdit.id = id
	}
	s.lock.Unlock()
	if update(&toEdit) {
		toEdit.id = id // We don't allow to mess with that one :)
		s.lock.Lock()
		defer s.lock.Unlock()
		if s.id2Component[id] != found {
			return false, "Editing conflict. Discarding this edit. Sorry."
		}
		s.id2Component[id] = &toEdit
		return true, ""
	}
	return true, ""
}

func (s *InMemoryStore) FindById(id int) *Component {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.id2Component[id]
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
	return s[i].score > s[j].score
}

func (s *InMemoryStore) Search(search_term string) []*Component {
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
