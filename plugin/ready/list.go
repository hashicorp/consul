package ready

import (
	"sort"
	"strings"
	"sync"
)

// list is a structure that holds the plugins that signals readiness for this server block.
type list struct {
	sync.RWMutex
	rs    []Readiness
	names []string
}

// Append adds a new readiness to l.
func (l *list) Append(r Readiness, name string) {
	l.Lock()
	defer l.Unlock()
	l.rs = append(l.rs, r)
	l.names = append(l.names, name)
}

// Ready return true when all plugins ready, if the returned value is false the string
// contains a comma separated list of plugins that are not ready.
func (l *list) Ready() (bool, string) {
	l.RLock()
	defer l.RUnlock()
	ok := true
	s := []string{}
	for i, r := range l.rs {
		if r == nil {
			continue
		}
		if !r.Ready() {
			ok = false
			s = append(s, l.names[i])
		} else {
			// if ok, this plugin is ready and will not be queried anymore.
			l.rs[i] = nil
		}
	}
	if ok {
		return true, ""
	}
	sort.Strings(s)
	return false, strings.Join(s, ",")
}
