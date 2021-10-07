package proxy

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

type Snapshot struct {
	mu   sync.RWMutex
	Data map[string]*URLData `yaml:"data"`
	used map[string]struct{}
}

func NewSnapshot() *Snapshot {
	return &Snapshot{
		Data: make(map[string]*URLData),
		used: make(map[string]struct{}),
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
		if err := yaml.Unmarshal(b, &data); err != nil {
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
		if err := yaml.Unmarshal(b, &data); err != nil {
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
	s.used[key] = struct{}{}
	return s.Data[key]
}

func (s *Snapshot) Set(key string, data *URLData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.used[key] = struct{}{}
	if _, ok := s.Data[key]; !ok {
		// First seen, save
		s.Data[key] = data
	} else if data.StatusCode != http.StatusNotModified {
		// Don't replace the initial request with a noop
		s.Data[key] = data
	}
}

func (s *Snapshot) Empty() bool {
	return s.Size() == 0
}

func (s *Snapshot) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Data)
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

	var activeData map[string]*URLData
	if len(s.used) > 0 {
		activeData = make(map[string]*URLData, len(s.Data))
		for k, v := range s.Data {
			if _, ok := s.used[k]; !ok {
				continue
			}
			activeData[k] = v
		}
	} else {
		activeData = s.Data
	}

	b, err := yaml.Marshal(activeData)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(fn, b, 0600)
}
