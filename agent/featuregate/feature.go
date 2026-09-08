// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

// Package featuregate owns the common registry contract, definitions, and
// resolution rules for dynamic experimental feature gates.
//
// Features are safe by default: define fills in the experimental stage, a
// disabled default, and disabled behavior before the minimum version. Adding a
// feature therefore requires only a definition here and a check at the generic
// behavior boundary; it does not require feature-specific control-plane
// plumbing.
package featuregate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"github.com/hashicorp/go-version"
)

type Stage string

const StageExperimental Stage = "experimental"

type BeforeMinimumVersionBehavior string

const BeforeMinimumVersionDisabled BeforeMinimumVersionBehavior = "disabled"

// Feature is an opaque token used by runtime consumers. Only definitions in
// this package can construct one.
type Feature struct {
	name string
}

func (f Feature) String() string { return f.name }

// Definition is immutable metadata compiled into every Consul binary.
type Definition struct {
	Name                 string
	Stage                Stage
	MinVersion           *version.Version
	DefaultEnabled       bool
	BeforeMinimumVersion BeforeMinimumVersionBehavior
	Description          string
	Owner                string
}

// Registry is the read-only feature-definition contract used by configuration,
// reconciliation, APIs, and runtime consumers. CE and Enterprise register
// concrete definitions into the same implementation from edition-specific
// registration files, following the HTTP endpoint registration pattern.
type Registry interface {
	Definitions() []Definition
	DefinitionForName(name string) (Definition, bool)
	Digest() string
}

var validName = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func normalizeDefinition(d Definition) Definition {
	if d.Stage == "" {
		d.Stage = StageExperimental
	}
	if d.BeforeMinimumVersion == "" {
		d.BeforeMinimumVersion = BeforeMinimumVersionDisabled
	}
	return d
}

func (d Definition) validate() error {
	if !validName.MatchString(d.Name) {
		return fmt.Errorf("name %q must be lower-case kebab-case", d.Name)
	}
	if d.Stage != StageExperimental {
		return fmt.Errorf("feature %q has unsupported stage %q", d.Name, d.Stage)
	}
	if d.MinVersion == nil {
		return fmt.Errorf("feature %q must declare a minimum Consul version", d.Name)
	}
	if d.BeforeMinimumVersion != BeforeMinimumVersionDisabled {
		return fmt.Errorf("feature %q has unsupported pre-minimum-version behavior %q", d.Name, d.BeforeMinimumVersion)
	}
	if d.Description == "" {
		return fmt.Errorf("feature %q must have a description", d.Name)
	}
	if d.Owner == "" {
		return fmt.Errorf("feature %q must have an owner", d.Name)
	}
	return nil
}

type registry struct {
	definitions map[string]Definition
}

var _ Registry = (*registry)(nil)

func newRegistry() *registry {
	return &registry{definitions: make(map[string]Definition)}
}

func (r *registry) define(d Definition) Feature {
	d = normalizeDefinition(d)
	if err := d.validate(); err != nil {
		panic(fmt.Sprintf("featuregate: invalid definition: %v", err))
	}
	if _, ok := r.definitions[d.Name]; ok {
		panic(fmt.Sprintf("featuregate: duplicate feature definition %q", d.Name))
	}
	r.definitions[d.Name] = d
	return Feature{name: d.Name}
}

func (r *registry) Definitions() []Definition {
	definitions := make([]Definition, 0, len(r.definitions))
	for _, definition := range r.definitions {
		definitions = append(definitions, definition)
	}
	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].Name < definitions[j].Name
	})
	return definitions
}

func (r *registry) DefinitionForName(name string) (Definition, bool) {
	d, ok := r.definitions[name]
	return d, ok
}

var defaultRegistry = newRegistry()

// DefaultRegistry returns the process-wide registry populated during package
// initialization by common and edition-specific registration files.
func DefaultRegistry() Registry {
	return defaultRegistry
}

// registerFeature is intentionally package-private. Common registrations live
// in feature_register.go; Enterprise can append registrations from a
// consulent-tagged file without exposing mutation to consumers.
func registerFeature(d Definition) Feature {
	return defaultRegistry.define(d)
}

// Digest identifies the exact set of definition metadata understood by
// this binary. It is used for reconciliation diagnostics, not compatibility
// negotiation.
func (r *registry) Digest() string {
	type digestDefinition struct {
		Name                 string
		Stage                Stage
		MinVersion           string
		DefaultEnabled       bool
		BeforeMinimumVersion BeforeMinimumVersionBehavior
		Description          string
		Owner                string
	}

	definitions := r.Definitions()
	digestInput := make([]digestDefinition, 0, len(definitions))
	for _, definition := range definitions {
		digestInput = append(digestInput, digestDefinition{
			Name:                 definition.Name,
			Stage:                definition.Stage,
			MinVersion:           definition.MinVersion.String(),
			DefaultEnabled:       definition.DefaultEnabled,
			BeforeMinimumVersion: definition.BeforeMinimumVersion,
			Description:          definition.Description,
			Owner:                definition.Owner,
		})
	}

	encoded, err := json.Marshal(digestInput)
	if err != nil {
		panic(fmt.Sprintf("featuregate: failed to encode registry digest: %v", err))
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
