package test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// For a given product and version (eg. vault and 1.13.2) you'll get the
// URL to download the binary (zipped) for AMD64 (hardcoded for now).
func downloadURL(product, version string) string {
	releaseURL := "https://api.releases.hashicorp.com/v1/releases/%s/%s"
	resp, err := http.Get(fmt.Sprintf(releaseURL, product, version))
	if err != nil {
		panic(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()

	type build struct {
		Arch, OS, URL string
	}
	type releasesJSON struct {
		Builds []build
	}
	var rel releasesJSON
	if err := json.Unmarshal(body, &rel); err != nil {
		panic(err)
	}

	for _, b := range rel.Builds {
		if b.Arch == "amd64" {
			return b.URL
		}
	}

	panic("No binary architecture match found.")
}

// loop through releases until a duplicate minor release number is found
// this should include all the currently supported releases
func latestReleases(product string) []string {
	lastTwenty := "https://api.releases.hashicorp.com/v1/releases/%s?license_class=oss&limit=20"
	resp, err := http.Get(fmt.Sprintf(lastTwenty, product))
	if err != nil {
		panic(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()

	type jsonVersion struct {
		Version string
	}
	var versions []jsonVersion
	if err := json.Unmarshal(body, &versions); err != nil {
		panic(err)
	}

	unique := make(map[string]struct{}, 3)
	result := []string{}
	for _, v := range versions {
		vl := strings.Split(v.Version, ".")
		minor := vl[1]
		if _, ok := unique[minor]; ok {
			break
		}
		unique[minor] = struct{}{}
		result = append(result, v.Version)
	}

	return result
}
