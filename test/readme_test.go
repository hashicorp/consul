package test

import (
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/coredns/coredns/core/dnsserver"

	"github.com/caddyserver/caddy"
)

// As we use the filesystem as-is, these files need to exist ON DISK for the readme test to work. This is especially
// useful for the *file* and *dnssec* plugins as their Corefiles are now tested as well. We create files in the
// current dir for all these, meaning the example READMEs MUST use relative path in their READMEs.
var contents = map[string]string{
	"Kexample.org.+013+45330.key":     examplePub,
	"Kexample.org.+013+45330.private": examplePriv,
	"example.org.signed":              exampleOrg, // not signed, but does not matter for this test.
}

const (
	examplePub = `example.org. IN DNSKEY 256 3 13 eNMYFZYb6e0oJOV47IPo5f/UHy7wY9aBebotvcKakIYLyyGscBmXJQhbKLt/LhrMNDE2Q96hQnI5PdTBeOLzhQ==
`
	examplePriv = `Private-key-format: v1.3
Algorithm: 13 (ECDSAP256SHA256)
PrivateKey: f03VplaIEA+KHI9uizlemUSbUJH86hPBPjmcUninPoM=
`
)

// TestReadme parses all README.mds of the plugins and checks if every example Corefile
// actually works. Each corefile snippet is only used if the language is set to 'corefile':
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

	create(contents)
	defer remove(contents)

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
			temp += line + "\n" // read newline stripped by s.Text()
		}
	}

	if err := s.Err(); err != nil {
		return nil, err
	}
	return input, nil
}

func create(c map[string]string) {
	for name, content := range c {
		ioutil.WriteFile(name, []byte(content), 0644)
	}
}

func remove(c map[string]string) {
	for name := range c {
		os.Remove(name)
	}
}
