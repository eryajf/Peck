package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// StateStore persists update information separately from user preferences.
type StateStore struct {
	path string
	mu   sync.Mutex
	data stateFile
}

type stateFile struct {
	LastCheck    *Result     `json:"lastCheck,omitempty"`
	ReadyToApply *ReadyState `json:"readyToApply,omitempty"`
}

type ReadyState struct {
	Version string `json:"version"`
	Path    string `json:"path"`
	Asset   string `json:"asset"`
}

func NewStateStore(path string) *StateStore {
	store := &StateStore{path: path}
	if path == "" {
		return store
	}
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &store.data)
	}
	return store
}

func (s *StateStore) LastCheck() (Result, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.LastCheck == nil {
		return Result{}, false
	}
	return *s.data.LastCheck, true
}

// TakeApplyFailure reads the helper's persisted failure for a pending package.
// The record is only consumed after a successful read.
func (s *StateStore) TakeApplyFailure() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.ReadyToApply == nil || s.data.ReadyToApply.Path == "" {
		return "", nil
	}
	path := s.data.ReadyToApply.Path + ".error"
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *StateStore) Ready() (ReadyState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.ReadyToApply == nil {
		return ReadyState{}, false
	}
	if _, err := os.Stat(s.data.ReadyToApply.Path); err != nil {
		return ReadyState{}, false
	}
	return *s.data.ReadyToApply, true
}

// ClearReady drops a stale pending update without deleting any user files.
func (s *StateStore) ClearReady() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.ReadyToApply = nil
	if s.path == "" {
		return nil
	}
	return s.persistLocked()
}

func (s *StateStore) SaveReady(ready ReadyState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return nil
	}
	s.data.ReadyToApply = &ready
	return s.persistLocked()
}

func (s *StateStore) SaveLastCheck(result Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" {
		return nil
	}
	s.data.LastCheck = &result
	return s.persistLocked()
}

func (s *StateStore) persistLocked() error {
	payload, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, payload, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, s.path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
