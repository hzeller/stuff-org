// Somewhat abandoned.
package main

import (
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
		toEdit.Id = id
	}
	s.lock.Unlock()
	if update(&toEdit) {
		if toEdit.Id != id {
			return false, "ID different after editing. Can't do that!"
		}
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
