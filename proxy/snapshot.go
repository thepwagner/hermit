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

	blobDir string
}

func NewSnapshot(blobDir string) *Snapshot {
	return &Snapshot{
		blobDir: blobDir,
		Data:    make(map[string]*URLData),
	}
}

func LoadSnapshot(blobDir string, index string) (*Snapshot, error) {
	b, err := ioutil.ReadFile(index)
	if err != nil {
		return nil, err
	}
	var data map[string]*URLData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	s := NewSnapshot(blobDir)
	s.Data = data
	return s, nil
}

func (s *Snapshot) Get(key string) *URLData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Data[key]
}

func (s *Snapshot) Content(data *URLData) ([]byte, error) {
	return ioutil.ReadFile(filepath.Join(s.blobDir, fmt.Sprintf("%x", data.Sha256)))
}

func (s *Snapshot) Set(key string, data *URLData, content []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Data[key] = data

	h := sha256.New()
	h.Write(content)
	data.Sha256 = fmt.Sprintf("%x", h.Sum(nil))
	return ioutil.WriteFile(filepath.Join(s.blobDir, data.Sha256), content, 0600)
}

func (s *Snapshot) Save(dir string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, err := json.Marshal(s.Data)
	if err != nil {
		return err
	}

	h := sha256.New()
	h.Write(b)
	sha := h.Sum(nil)

	fn := filepath.Join(dir, fmt.Sprintf("%x.json", sha))
	return ioutil.WriteFile(fn, b, 0600)
}

type URLData struct {
	ContentType string `json:"contentType"`
	Sha256      string `json:"sha256"`
}
