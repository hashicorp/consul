package auto

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/coredns/coredns/plugin/file"

	"github.com/miekg/dns"
)

// Walk will recursively walk of the file under l.directory and adds the one that match l.re.
func (a Auto) Walk() error {

	// TODO(miek): should add something so that we don't stomp on each other.

	toDelete := make(map[string]bool)
	for _, n := range a.Zones.Names() {
		toDelete[n] = true
	}

	filepath.Walk(a.loader.directory, func(path string, info os.FileInfo, _ error) error {
		if info == nil || info.IsDir() {
			return nil
		}

		match, origin := matches(a.loader.re, info.Name(), a.loader.template)
		if !match {
			return nil
		}

		if z, ok := a.Zones.Z[origin]; ok {
			// we already have this zone
			toDelete[origin] = false
			z.SetFile(path)
			return nil
		}

		reader, err := os.Open(path)
		if err != nil {
			log.Warningf("Opening %s failed: %s", path, err)
			return nil
		}
		defer reader.Close()

		// Serial for loading a zone is 0, because it is a new zone.
		zo, err := file.Parse(reader, origin, path, 0)
		if err != nil {
			log.Warningf("Parse zone `%s': %v", origin, err)
			return nil
		}

		zo.ReloadInterval = a.loader.ReloadInterval
		zo.Upstream = a.loader.upstream
		zo.TransferTo = a.loader.transferTo

		a.Zones.Add(zo, origin)

		if a.metrics != nil {
			a.metrics.AddZone(origin)
		}

		zo.Notify()

		log.Infof("Inserting zone `%s' from: %s", origin, path)

		toDelete[origin] = false

		return nil
	})

	for origin, ok := range toDelete {
		if !ok {
			continue
		}

		if a.metrics != nil {
			a.metrics.RemoveZone(origin)
		}

		a.Zones.Remove(origin)

		log.Infof("Deleting zone `%s'", origin)
	}

	return nil
}

// matches re to filename, if it is a match, the subexpression will be used to expand
// template to an origin. When match is true that origin is returned. Origin is fully qualified.
func matches(re *regexp.Regexp, filename, template string) (match bool, origin string) {
	base := filepath.Base(filename)

	matches := re.FindStringSubmatchIndex(base)
	if matches == nil {
		return false, ""
	}

	by := re.ExpandString(nil, template, base, matches)
	if by == nil {
		return false, ""
	}

	origin = dns.Fqdn(string(by))

	return true, origin
}
