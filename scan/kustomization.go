package scan

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type KustomizeImage struct {
	Name    string `yaml:"name"`
	NewName string `yaml:"newName"`
	NewTag  string `yaml:"newTag"`
}

func (i KustomizeImage) Image() string {
	if i.NewName != "" {
		return fmt.Sprintf("%s:%s", i.NewName, i.NewTag)
	} else {
		return fmt.Sprintf("%s:%s", i.Name, i.NewTag)
	}
}

type Kustomization struct {
	Path   string           `yaml:"-"`
	Images []KustomizeImage `yaml:"images"`
}

func KustomizationFile(path string) (*Kustomization, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	k, err := ParseKustomization(f)
	if err != nil {
		return nil, err
	}
	k.Path = path
	return k, nil
}

func ParseKustomization(r io.Reader) (*Kustomization, error) {
	var k Kustomization
	if err := yaml.NewDecoder(r).Decode(&k); err != nil {
		return nil, err
	}
	return &k, nil
}

type Kustomizations []*Kustomization

func WalkKustomizations(root string) (Kustomizations, error) {
	var ret Kustomizations
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Base(path) != "kustomization.yaml" {
			return nil
		}
		k, err := KustomizationFile(path)
		if err != nil {
			return err
		}
		ret = append(ret, k)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (ks Kustomizations) Images() []KustomizeImage {
	var ret []KustomizeImage
	index := make(map[string]struct{})
	for _, k := range ks {
		for _, i := range k.Images {
			img := i.Image()
			if _, ok := index[img]; !ok {
				ret = append(ret, i)
				index[img] = struct{}{}
			}
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Image() < ret[j].Image()
	})
	return ret
}
