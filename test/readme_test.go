package test

import (
	"bufio"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/coredns/coredns/core/dnsserver"

	"github.com/mholt/caddy"
)

// Pasrse all README.md's of the plugin and check if every example Corefile
// actually works. Each corefile is only used if the language is set to 'corefile':
//
// ~~~ corefile
// . {
//	# check-this-please
// }
// ~~~

func TestReadme(t *testing.T) {
	port := 30053
	caddy.Quiet = true
	dnsserver.Quiet = true

	log.SetOutput(ioutil.Discard)

	middle := filepath.Join("..", "plugin")
	dirs, err := ioutil.ReadDir(middle)
	if err != nil {
		t.Fatalf("Could not read %s: %q", middle, err)
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		readme := filepath.Join(middle, d.Name())
		readme = filepath.Join(readme, "README.md")

		inputs, err := corefileFromReadme(readme)
		if err != nil {
			continue
		}

		// Test each snippet.
		for _, in := range inputs {
			dnsserver.Port = strconv.Itoa(port)
			server, err := caddy.Start(in)
			if err != nil {
				t.Errorf("Failed to start server with %s, for input %q:\n%s", readme, err, in.Body())
			}
			server.Stop()
			port++
		}
	}
}

// corefileFromReadme parses a readme and returns all fragments that
// have ~~~ corefile (or ``` corefile).
func corefileFromReadme(readme string) ([]*Input, error) {
	f, err := os.Open(readme)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	input := []*Input{}
	corefile := false
	temp := ""

	for s.Scan() {
		line := s.Text()
		if line == "~~~ corefile" || line == "``` corefile" {
			corefile = true
			continue
		}

		if corefile && (line == "~~~" || line == "```") {
			// last line
			input = append(input, NewInput(temp))

			temp = ""
			corefile = false
			continue
		}

		if corefile {
			temp += line + "\n" // readd newline stripped by s.Text()
		}
	}

	if err := s.Err(); err != nil {
		return nil, err
	}
	return input, nil
}
