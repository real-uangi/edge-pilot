package application

import "sync"

type RuntimeState struct {
	mu      sync.RWMutex
	running map[string]struct{}
}

func NewRuntimeState() *RuntimeState {
	return &RuntimeState{
		running: make(map[string]struct{}),
	}
}

func (s *RuntimeState) Start(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.running[taskID] = struct{}{}
}

func (s *RuntimeState) TryStart(taskID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.running[taskID]; ok {
		return false
	}
	s.running[taskID] = struct{}{}
	return true
}

func (s *RuntimeState) Done(taskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, taskID)
}

func (s *RuntimeState) RunningTaskIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]string, 0, len(s.running))
	for id := range s.running {
		ids = append(ids, id)
	}
	return ids
}
