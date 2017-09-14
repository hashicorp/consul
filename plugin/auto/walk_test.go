package auto

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"regexp"
	"testing"
)

var dbFiles = []string{"db.example.org", "aa.example.org"}

const zoneContent = `; testzone
@	IN	SOA	sns.dns.icann.org. noc.dns.icann.org. 2016082534 7200 3600 1209600 3600
		NS	a.iana-servers.net.
		NS	b.iana-servers.net.

www IN A 127.0.0.1
`

func TestWalk(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	tempdir, err := createFiles()
	if err != nil {
		if tempdir != "" {
			os.RemoveAll(tempdir)
		}
		t.Fatal(err)
	}
	defer os.RemoveAll(tempdir)

	ldr := loader{
		directory: tempdir,
		re:        regexp.MustCompile(`db\.(.*)`),
		template:  `${1}`,
	}

	a := Auto{
		loader: ldr,
		Zones:  &Zones{},
	}

	a.Walk()

	// db.example.org and db.example.com should be here (created in createFiles)
	for _, name := range []string{"example.com.", "example.org."} {
		if _, ok := a.Zones.Z[name]; !ok {
			t.Errorf("%s should have been added", name)
		}
	}
}

func TestWalkNonExistent(t *testing.T) {
	log.SetOutput(ioutil.Discard)

	nonExistingDir := "highly_unlikely_to_exist_dir"

	ldr := loader{
		directory: nonExistingDir,
		re:        regexp.MustCompile(`db\.(.*)`),
		template:  `${1}`,
	}

	a := Auto{
		loader: ldr,
		Zones:  &Zones{},
	}

	a.Walk()
}

func createFiles() (string, error) {
	dir, err := ioutil.TempDir(os.TempDir(), "coredns")
	if err != nil {
		return dir, err
	}

	for _, name := range dbFiles {
		if err := ioutil.WriteFile(path.Join(dir, name), []byte(zoneContent), 0644); err != nil {
			return dir, err
		}
	}
	// symlinks
	if err = os.Symlink(path.Join(dir, "db.example.org"), path.Join(dir, "db.example.com")); err != nil {
		return dir, err
	}
	if err = os.Symlink(path.Join(dir, "db.example.org"), path.Join(dir, "aa.example.com")); err != nil {
		return dir, err
	}

	return dir, nil
}
