// Package rewrite is middleware for rewriting requests internally to something different.
package rewrite

// Fields defines additional FIELD keywords may be implemented to support more rewrite use-cases.
// New Rule types must be added to the Fields map.
// The type must implement `New` and `Rewrite` functions.
var Fields = map[string]Rule{
	"name":  NameRule{},
	"type":  TypeRule{},
	"class": ClassRule{},
}
