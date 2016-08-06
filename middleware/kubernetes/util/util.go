// Package kubernetes/util provides helper functions for the kubernetes middleware
package util

import (
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
)

// StringInSlice check whether string a is a member of slice.
func StringInSlice(a string, slice []string) bool {
	for _, b := range slice {
		if b == a {
			return true
		}
	}
	return false
}

// SymbolContainsWildcard checks whether symbol contains a wildcard value
func SymbolContainsWildcard(symbol string) bool {
	return (strings.Contains(symbol, WildcardStar) || (symbol == WildcardAny))
}

const (
	WildcardStar = "*"
	WildcardAny  = "any"
)

// StoreToNamespaceLister makes a Store that lists Namespaces.
type StoreToNamespaceLister struct {
	cache.Store
}

// List lists all Namespaces in the store.
func (s *StoreToNamespaceLister) List() (ns api.NamespaceList, err error) {
	for _, m := range s.Store.List() {
		ns.Items = append(ns.Items, *(m.(*api.Namespace)))
	}
	return ns, nil
}
