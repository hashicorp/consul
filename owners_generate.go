//+build ignore

// generates plugin/chaos/zowners.go.

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v2"
)

func main() {
	o := map[string]struct{}{}

	// top-level OWNERS file
	o, err := owners("OWNERS", o)
	if err != nil {
		log.Fatal(err)
	}

	// each of the plugins, in case someone is not in the top-level one
	err = filepath.Walk("plugin",
		func(p string, i os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if i.IsDir() {
				return nil
			}
			if path.Base(p) != "OWNERS" {
				return nil
			}
			o, err = owners(p, o)
			return err
		})

	// sort it and format it
	list := []string{}
	for k := range o {
		list = append(list, k)
	}
	sort.Strings(list)
	golist := `package chaos

// Owners are all GitHub handlers of all maintainers.
var Owners = []string{`
	c := ", "
	for i, a := range list {
		if i == len(list)-1 {
			c = "}"
		}
		golist += fmt.Sprintf("%q%s", a, c)
	}

	if err := ioutil.WriteFile("plugin/chaos/zowners.go", []byte(golist), 0644); err != nil {
		log.Fatal(err)
	}
	return
}

// owners parses a owner file without knowning a whole lot about its structure.
func owners(path string, owners map[string]struct{}) (map[string]struct{}, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := yaml.MapSlice{}
	err = yaml.Unmarshal(file, &c)
	if err != nil {
		return nil, err
	}
	for _, mi := range c {
		key, ok := mi.Key.(string)
		if !ok {
			continue
		}
		if key == "approvers" {
			for _, k := range mi.Value.([]interface{}) {
				owners[k.(string)] = struct{}{}
			}
		}
	}
	return owners, nil
}
