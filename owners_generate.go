//+build ignore

// generates plugin/chaos/zowners.go.

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strings"
)

func main() {
	// top-level OWNERS file
	o, err := owners("CODEOWNERS")
	if err != nil {
		log.Fatal(err)
	}

	golist := `package chaos

// Owners are all GitHub handlers of all maintainers.
var Owners = []string{`
	c := ", "
	for i, a := range o {
		if i == len(o)-1 {
			c = "}"
		}
		golist += fmt.Sprintf("%q%s", a, c)
	}
	// to prevent `No newline at end of file` with gofmt
	golist += "\n"

	if err := ioutil.WriteFile("plugin/chaos/zowners.go", []byte(golist), 0644); err != nil {
		log.Fatal(err)
	}
	return
}

func owners(path string) ([]string, error) {
	// simple line, by line based format
	//
	// # In this example, @doctocat owns any files in the build/logs
	// # directory at the root of the repository and any of its
	// # subdirectories.
	// /build/logs/ @doctocat
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(f)
	users := map[string]struct{}{}
	for scanner.Scan() {
		text := scanner.Text()
		if len(text) == 0 {
			continue
		}
		if text[0] == '#' {
			continue
		}
		ele := strings.Fields(text)
		if len(ele) == 0 {
			continue
		}

		// ok ele[0] is the path, the rest are (in our case) github usernames prefixed with @
		for _, s := range ele[1:] {
			if len(s) <= 1 {
				continue
			}
			users[s[1:]] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	u := []string{}
	for k := range users {
		if strings.HasPrefix(k, "@") {
			k = k[1:]
		}
		u = append(u, k)
	}
	sort.Strings(u)
	return u, nil
}
