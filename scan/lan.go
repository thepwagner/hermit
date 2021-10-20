package scan

import (
	"fmt"
	"strings"
)

const (
	lanRegistry        = "registry.k8s.pwagner.net"
	dockerhubProxyName = "dockerhub"
)

// LanImage returns the reference to an image through my local registry
func LanImage(img string) string {
	if strings.ContainsAny(img, " &;|") {
		return ""
	}

	s := strings.Split(img, "/")
	switch len(s) {
	case 1:
		return fmt.Sprintf("%s/%s/library/%s", lanRegistry, dockerhubProxyName, img)
	case 2:
		return fmt.Sprintf("%s/%s/%s", lanRegistry, dockerhubProxyName, img)
	default:
		if strings.HasPrefix(img, "quay.io/") {
			return fmt.Sprintf("%s/quay/%s", lanRegistry, strings.Replace(img, "quay.io/", "", 1))
		}
		return img
	}
}
