package proxy

import (
	lru "github.com/hashicorp/golang-lru"
)

type LRUStorage struct {
	lru    *lru.Cache
	source Storage
}

func NewLRUStorage(size int, source Storage) (*LRUStorage, error) {
	lru, err := lru.New(size)
	if err != nil {
		return nil, err
	}
	return &LRUStorage{lru: lru, source: source}, nil
}

func (s *LRUStorage) Load(d *URLData) ([]byte, error) {
	cached, ok := s.lru.Get(d.Sha256)
	if ok {
		return cached.([]byte), nil
	}

	b, err := s.source.Load(d)
	if err != nil {
		return nil, err
	}
	s.lru.Add(d.Sha256, b)
	return b, nil
}

func (s *LRUStorage) Store(data *URLData, content []byte) error {
	s.lru.Add(data.Sha256, content)
	return s.source.Store(data, content)
}
