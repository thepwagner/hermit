package proxy

import (
	"io/ioutil"
	"path/filepath"

	"github.com/go-logr/logr"
)

type Storage interface {
	Load(*URLData) ([]byte, error)
	Store(data *URLData, content []byte) error
}

type FileStorage struct {
	log     logr.Logger
	blobDir string
}

var _ Storage = (*FileStorage)(nil)

func NewFileStorage(log logr.Logger, blobDir string) *FileStorage {
	return &FileStorage{
		log:     log,
		blobDir: blobDir,
	}
}

func (s *FileStorage) Load(data *URLData) ([]byte, error) {
	p := filepath.Join(s.blobDir, data.Sha256)
	s.log.Info("load content", "path", p)
	return ioutil.ReadFile(p)
}

func (s *FileStorage) Store(data *URLData, content []byte) error {
	p := s.path(data)
	s.log.Info("store content", "path", p)
	return ioutil.WriteFile(p, content, 0600)
}

func (s *FileStorage) path(d *URLData) string {
	return filepath.Join(s.blobDir, d.Sha256)
}
