package test

import (
	"bufio"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/coredns/coredns/core/dnsserver"

	"github.com/mholt/caddy"
)

// Pasrse all README.md's of the middleware and check if every example Corefile
// actually works. Each corefile is only used if:
//
// ~~~ corefile
// . {
//	# check-this-please
// }
// ~~~

func TestReadme(t *testing.T) {
	caddy.Quiet = true
	dnsserver.Quiet = true
	dnsserver.Port = "10053"
	log.SetOutput(ioutil.Discard)

	middle := filepath.Join("..", "middleware")
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
			t.Logf("Testing %s, with %d byte snippet", readme, len(in.Body()))
			server, err := caddy.Start(in)
			if err != nil {
				t.Errorf("Failed to start server for input %q:\n%s", err, in.Body())
			}
			server.Stop()
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
