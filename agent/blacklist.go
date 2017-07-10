package agent

import (
	"github.com/armon/go-radix"
)

// Blacklist implements an HTTP endpoint blacklist based on a list of endpoint
// prefixes which should be disallowed.
type Blacklist struct {
	tree *radix.Tree
}

// NewBlacklist returns a blacklist for the given list of prefixes.
func NewBlacklist(prefixes []string) *Blacklist {
	tree := radix.New()
	for _, prefix := range prefixes {
		tree.Insert(prefix, nil)
	}
	return &Blacklist{tree}
}

// Block will return true if the given path is included among any of the
// block prefixes.
func (b *Blacklist) Block(path string) bool {
	_, _, blocked := b.tree.LongestPrefix(path)
	return blocked
}
