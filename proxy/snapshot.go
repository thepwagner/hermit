package proxy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sync"
)

type Snapshot struct {
	mu   sync.RWMutex
	Data map[string]*URLData `json:"data"`
}

func NewSnapshot() *Snapshot {
	return &Snapshot{
		Data: make(map[string]*URLData),
	}
}

func LoadSnapshot(index string) (*Snapshot, error) {
	b, err := ioutil.ReadFile(index)
	if err != nil {
		return nil, err
	}
	var data map[string]*URLData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	s := NewSnapshot()
	s.Data = data
	return s, nil
}

func (s *Snapshot) Get(key string) *URLData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Data[key]
}

func (s *Snapshot) Set(key string, data *URLData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data[key] = data
}

// Save writes this snapshot to a unique filename within the given directory
func (s *Snapshot) Save(dir string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, err := json.Marshal(s.Data)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	h.Write(b)
	sha := h.Sum(nil)

	fn := filepath.Join(dir, fmt.Sprintf("%x.json", sha))
	if err := ioutil.WriteFile(fn, b, 0600); err != nil {
		return "", err
	}
	return fn, nil
}
