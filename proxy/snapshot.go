package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
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
	s := NewSnapshot()
	if index == "" {
		return s, nil
	}

	stat, err := os.Stat(index)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	if !stat.IsDir() {
		b, err := ioutil.ReadFile(index)
		if err != nil {
			return nil, err
		}
		var data map[string]*URLData
		if err := json.Unmarshal(b, &data); err != nil {
			return nil, err
		}
		s.Data = data
		return s, nil
	}

	dir, err := ioutil.ReadDir(index)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, f := range dir {
		b, err := ioutil.ReadFile(filepath.Join(index, f.Name()))
		if err != nil {
			return nil, err
		}
		var data map[string]*URLData
		if err := json.Unmarshal(b, &data); err != nil {
			return nil, err
		}
		for k, v := range data {
			s.Data[k] = v
		}
	}
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

func (s *Snapshot) Empty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Data) == 0
}

func (s *Snapshot) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data = make(map[string]*URLData)
}

func (s *Snapshot) ByHost() map[string]map[string]*URLData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byHost := make(map[string]map[string]*URLData)
	for k, v := range s.Data {
		u, _ := url.Parse(fmt.Sprintf("https://%s", k))
		if existing, ok := byHost[u.Host]; ok {
			existing[k] = v
		} else {
			byHost[u.Host] = map[string]*URLData{k: v}
		}
	}
	return byHost
}

// Save writes this snapshot to a unique filename within the given directory
func (s *Snapshot) Save(fn string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, err := json.Marshal(s.Data)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fn, b, 0600)
}
