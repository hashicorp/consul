// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package structs

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul/acl"
)

type ServiceIntentionsConfigEntry struct {
	Kind string
	Name string // formerly DestinationName

	Sources []*SourceIntention

	JWT *IntentionJWTRequirement `json:",omitempty"`

	Meta map[string]string `json:",omitempty"` // formerly Intention.Meta

	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"` // formerly DestinationNS
	RaftIndex
}

var _ UpdatableConfigEntry = (*ServiceIntentionsConfigEntry)(nil)

func (e *ServiceIntentionsConfigEntry) GetKind() string {
	return ServiceIntentions
}

func (e *ServiceIntentionsConfigEntry) GetName() string {
	if e == nil {
		return ""
	}

	return e.Name
}

func (e *ServiceIntentionsConfigEntry) GetMeta() map[string]string {
	if e == nil {
		return nil
	}
	return e.Meta
}

func (e *ServiceIntentionsConfigEntry) Clone() *ServiceIntentionsConfigEntry {
	e2 := *e

	e2.Meta = cloneStringStringMap(e.Meta)

	e2.Sources = make([]*SourceIntention, len(e.Sources))
	for i, src := range e.Sources {
		e2.Sources[i] = src.Clone()
	}

	if e.JWT != nil {
		e2.JWT = e.JWT.Clone()
	}

	return &e2
}

func (e *ServiceIntentionsConfigEntry) DestinationServiceName() ServiceName {
	return NewServiceName(e.Name, &e.EnterpriseMeta)
}

func (e *ServiceIntentionsConfigEntry) UpdateSourceByLegacyID(legacyID string, update *SourceIntention) bool {
	for i, src := range e.Sources {
		if src.LegacyID == legacyID {
			e.Sources[i] = update
			return true
		}
	}
	return false
}

func (e *ServiceIntentionsConfigEntry) UpsertSourceByName(sn ServiceName, upsert *SourceIntention) {
	for i, src := range e.Sources {
		if src.SourceServiceName() == sn {
			e.Sources[i] = upsert
			return
		}
	}

	e.Sources = append(e.Sources, upsert)
}

func (e *ServiceIntentionsConfigEntry) DeleteSourceByLegacyID(legacyID string) bool {
	for i, src := range e.Sources {
		if src.LegacyID == legacyID {
			// Delete slice element: https://github.com/golang/go/wiki/SliceTricks#delete
			//    a = append(a[:i], a[i+1:]...)
			e.Sources = append(e.Sources[:i], e.Sources[i+1:]...)

			if len(e.Sources) == 0 {
				e.Sources = nil
			}
			return true
		}
	}
	return false
}

func (e *ServiceIntentionsConfigEntry) DeleteSourceByName(sn ServiceName) bool {
	for i, src := range e.Sources {
		if src.SourceServiceName() == sn {
			// Delete slice element: https://github.com/golang/go/wiki/SliceTricks#delete
			//    a = append(a[:i], a[i+1:]...)
			e.Sources = append(e.Sources[:i], e.Sources[i+1:]...)

			if len(e.Sources) == 0 {
				e.Sources = nil
			}
			return true
		}
	}
	return false
}

func (e *ServiceIntentionsConfigEntry) ToIntention(src *SourceIntention) *Intention {
	meta := e.Meta
	if src.LegacyID != "" {
		meta = src.LegacyMeta
	}

	ixn := &Intention{
		ID:                   src.LegacyID,
		Description:          src.Description,
		SourcePeer:           src.Peer,
		SourceSamenessGroup:  src.SamenessGroup,
		SourcePartition:      src.PartitionOrEmpty(),
		SourceNS:             src.NamespaceOrDefault(),
		SourceName:           src.Name,
		SourceType:           src.Type,
		JWT:                  e.JWT,
		Action:               src.Action,
		Permissions:          src.Permissions,
		Meta:                 meta,
		Precedence:           src.Precedence,
		DestinationPartition: e.PartitionOrEmpty(),
		DestinationNS:        e.NamespaceOrDefault(),
		DestinationName:      e.Name,
		RaftIndex:            e.RaftIndex,
	}

	if src.LegacyCreateTime != nil {
		ixn.CreatedAt = *src.LegacyCreateTime
	}
	if src.LegacyUpdateTime != nil {
		ixn.UpdatedAt = *src.LegacyUpdateTime
	}

	if src.LegacyID != "" {
		// Ensure that pre-1.9.0 secondaries can still replicate legacy
		// intentions via the APIs. These require the Hash field to be
		// populated.
		//
		//nolint:staticcheck
		ixn.SetHash()
	}
	return ixn
}

func (e *ServiceIntentionsConfigEntry) LegacyIDFieldsAreAllEmpty() bool {
	for _, src := range e.Sources {
		if src.LegacyID != "" {
			return false
		}
	}
	return true
}

func (e *ServiceIntentionsConfigEntry) LegacyIDFieldsAreAllSet() bool {
	for _, src := range e.Sources {
		if src.LegacyID == "" {
			return false
		}
	}
	return true
}

func (e *ServiceIntentionsConfigEntry) ToIntentions() Intentions {
	out := make(Intentions, 0, len(e.Sources))
	for _, src := range e.Sources {
		out = append(out, e.ToIntention(src))
	}
	return out
}

type SourceIntention struct {
	// Name is the name of the source service. This can be a wildcard "*", but
	// only the full value can be a wildcard. Partial wildcards are not
	// allowed.
	//
	// The source may also be a non-Consul service, as specified by SourceType.
	//
	// formerly Intention.SourceName
	Name string

	// Action is whether this is an allowlist or denylist intention.
	//
	// formerly Intention.Action
	//
	// NOTE: this is mutually exclusive with the Permissions field.
	Action IntentionAction `json:",omitempty"`

	// Permissions is the list of additional L7 attributes that extend the
	// intention definition.
	//
	// Permissions are interpreted in the order represented in the slice. In
	// default-deny mode, deny permissions are logically subtracted from all
	// following allow permissions. Multiple allow permissions are then ORed
	// together.
	//
	// For example:
	//   ["deny /v2/admin", "allow /v2/*", "allow GET /healthz"]
	//
	// Is logically interpreted as:
	//   allow: [
	//     "(/v2/*) AND NOT (/v2/admin)",
	//     "(GET /healthz) AND NOT (/v2/admin)"
	//   ]
	Permissions []*IntentionPermission `json:",omitempty"`

	// Precedence is the order that the intention will be applied, with
	// larger numbers being applied first. This is a read-only field, on
	// any intention update it is updated.
	//
	// Note we will technically decode this over the wire during a write, but
	// we always recompute it on save.
	//
	// formerly Intention.Precedence
	Precedence int

	// LegacyID is manipulated just by the bridging code
	// used as part of backwards compatibility.
	//
	// formerly Intention.ID
	LegacyID string `json:",omitempty" alias:"legacy_id"`

	// Type is the type of the value for the source.
	//
	// formerly Intention.SourceType
	Type IntentionSourceType

	// Description is a human-friendly description of this intention.
	// It is opaque to Consul and is only stored and transferred in API
	// requests.
	//
	// formerly Intention.Description
	Description string `json:",omitempty"`

	// LegacyMeta is arbitrary metadata associated with the intention. This is
	// opaque to Consul but is served in API responses.
	//
	// formerly Intention.Meta
	LegacyMeta map[string]string `json:",omitempty" alias:"legacy_meta"`

	// LegacyCreateTime is formerly Intention.CreatedAt
	LegacyCreateTime *time.Time `json:",omitempty" alias:"legacy_create_time"`
	// LegacyUpdateTime is formerly Intention.UpdatedAt
	LegacyUpdateTime *time.Time `json:",omitempty" alias:"legacy_update_time"`

	// Things like L7 rules or Sentinel rules could go here later.

	// formerly Intention.SourceNS
	acl.EnterpriseMeta `hcl:",squash" mapstructure:",squash"`

	// Peer is the name of the remote peer of the source service, if applicable.
	Peer string `json:",omitempty"`

	// SamenessGroup is the name of the sameness group, if applicable.
	SamenessGroup string `json:",omitempty" alias:"sameness_group"`
}

type IntentionJWTRequirement struct {
	// Providers is a list of providers to consider when verifying a JWT.
	Providers []*IntentionJWTProvider `json:",omitempty"`
}

func (e *IntentionJWTRequirement) Clone() *IntentionJWTRequirement {
	e2 := *e

	e2.Providers = make([]*IntentionJWTProvider, len(e.Providers))
	for i, src := range e.Providers {
		e2.Providers[i] = src.Clone()
	}
	return &e2
}

func (p *IntentionJWTProvider) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("JWT provider name is required")
	}
	return nil
}

func (e *IntentionJWTRequirement) Validate() error {
	var result error

	for _, provider := range e.Providers {
		if err := provider.Validate(); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result
}

type IntentionJWTProvider struct {
	// Name is the name of the JWT provider. There MUST be a corresponding
	// "jwt-provider" config entry with this name.
	Name string `json:",omitempty"`

	// VerifyClaims is a list of additional claims to verify in a JWT's payload.
	VerifyClaims []*IntentionJWTClaimVerification `json:",omitempty" alias:"verify_claims"`
}

func (e *IntentionJWTProvider) Clone() *IntentionJWTProvider {
	e2 := *e

	e2.VerifyClaims = make([]*IntentionJWTClaimVerification, len(e.VerifyClaims))
	for i, src := range e.VerifyClaims {
		e2.VerifyClaims[i] = src.Clone()
	}
	return &e2
}

type IntentionJWTClaimVerification struct {
	// Path is the path to the claim in the token JSON.
	Path []string `json:",omitempty"`

	// Value is the expected value at the given path:
	// - If the type at the path is a list then we verify
	//   that this value is contained in the list.
	//
	// - If the type at the path is a string then we verify
	//   that this value matches.
	Value string `json:",omitempty"`
}

func (e *IntentionJWTClaimVerification) Clone() *IntentionJWTClaimVerification {
	e2 := *e

	e2.Path = stringslice.CloneStringSlice(e.Path)
	return &e2
}

type IntentionPermission struct {
	Action IntentionAction // required: allow|deny

	HTTP *IntentionHTTPPermission `json:",omitempty"`

	// If we have non-http match criteria for other protocols
	// in the future (gRPC, redis, etc) they can go here.

	// Support for edge-decoded JWTs would likely be configured
	// in a new top level section here.

	// If we ever add Sentinel support, this is one place we may
	// wish to add it.

	JWT *IntentionJWTRequirement `json:",omitempty"`
}

func (p *IntentionPermission) Clone() *IntentionPermission {
	p2 := *p
	if p.HTTP != nil {
		p2.HTTP = p.HTTP.Clone()
	}
	if p.JWT != nil {
		p2.JWT = p.JWT.Clone()
	}
	return &p2
}

func (p *IntentionPermission) Validate() error {
	var result error
	if p.JWT != nil {
		result = p.JWT.Validate()
	}

	return result
}

type IntentionHTTPPermission struct {
	// PathExact, PathPrefix, and PathRegex are mutually exclusive.
	PathExact  string `json:",omitempty" alias:"path_exact"`
	PathPrefix string `json:",omitempty" alias:"path_prefix"`
	PathRegex  string `json:",omitempty" alias:"path_regex"`

	Header []IntentionHTTPHeaderPermission `json:",omitempty"`

	Methods []string `json:",omitempty"`
}

func (p *IntentionHTTPPermission) Clone() *IntentionHTTPPermission {
	p2 := *p

	if len(p.Header) > 0 {
		p2.Header = make([]IntentionHTTPHeaderPermission, 0, len(p.Header))
		for _, hdr := range p.Header {
			p2.Header = append(p2.Header, hdr)
		}
	}

	p2.Methods = stringslice.CloneStringSlice(p.Methods)

	return &p2
}

type IntentionHTTPHeaderPermission struct {
	Name    string
	Present bool   `json:",omitempty"`
	Exact   string `json:",omitempty"`
	Prefix  string `json:",omitempty"`
	Suffix  string `json:",omitempty"`
	Regex   string `json:",omitempty"`
	Invert  bool   `json:",omitempty"`
}

func cloneStringStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	m2 := make(map[string]string)
	for k, v := range m {
		m2[k] = v
	}
	return m2
}

func (x *SourceIntention) SourceServiceName() ServiceName {
	return NewServiceName(x.Name, &x.EnterpriseMeta)
}

func (x *SourceIntention) Clone() *SourceIntention {
	x2 := *x

	x2.LegacyMeta = cloneStringStringMap(x.LegacyMeta)

	if len(x.Permissions) > 0 {
		x2.Permissions = make([]*IntentionPermission, 0, len(x.Permissions))
		for _, perm := range x.Permissions {
			x2.Permissions = append(x2.Permissions, perm.Clone())
		}
	}

	return &x2
}

func (e *ServiceIntentionsConfigEntry) UpdateOver(rawPrev ConfigEntry) error {
	if rawPrev == nil {
		return nil
	}

	prev, ok := rawPrev.(*ServiceIntentionsConfigEntry)
	if !ok {
		return fmt.Errorf("previous config entry is not of type %T: %T", e, rawPrev)
	}

	var (
		prevSourceByName     = make(map[PeeredServiceName]*SourceIntention)
		prevSourceByLegacyID = make(map[string]*SourceIntention)
	)
	for _, src := range prev.Sources {
		prevSourceByName[PeeredServiceName{Peer: src.Peer, ServiceName: src.SourceServiceName()}] = src
		if src.LegacyID != "" {
			prevSourceByLegacyID[src.LegacyID] = src
		}
	}

	for i, src := range e.Sources {
		if src.LegacyID == "" {
			continue
		}

		// Check that the LegacyID fields are handled correctly during updates.
		if prevSrc, ok := prevSourceByName[PeeredServiceName{Peer: src.Peer, ServiceName: src.SourceServiceName()}]; ok {
			if prevSrc.LegacyID == "" {
				return fmt.Errorf("Sources[%d].LegacyID: cannot set this field", i)
			} else if src.LegacyID != prevSrc.LegacyID {
				return fmt.Errorf("Sources[%d].LegacyID: cannot set this field to a different value", i)
			}
		}

		// Now ensure legacy timestamps carry over properly. We always retain the LegacyCreateTime.
		if prevSrc, ok := prevSourceByLegacyID[src.LegacyID]; ok {
			if prevSrc.LegacyCreateTime != nil {
				// NOTE: we don't want to share the memory here
				src.LegacyCreateTime = timePointer(*prevSrc.LegacyCreateTime)
			}
		}
	}

	return nil
}

func (e *ServiceIntentionsConfigEntry) Normalize() error {
	return e.normalize(false)
}

func (e *ServiceIntentionsConfigEntry) LegacyNormalize() error {
	return e.normalize(true)
}

func (e *ServiceIntentionsConfigEntry) normalize(legacyWrite bool) error {
	if e == nil {
		return fmt.Errorf("config entry is nil")
	}

	// NOTE: this function must be deterministic so that the raft log doesn't
	// diverge. This means no ID assignments or time.Now() usage!

	e.Kind = ServiceIntentions

	e.EnterpriseMeta.Normalize()

	for _, src := range e.Sources {
		// Default source type
		if src.Type == "" {
			src.Type = IntentionSourceConsul
		}

		// Normalize the source's namespace and partition.
		// If the source is not peered, it inherits the destination's
		// EnterpriseMeta.
		if src.Peer != "" || src.SamenessGroup != "" {
			// If the source is peered or a sameness group, normalize the namespace only,
			// since they are mutually exclusive with partition.
			src.EnterpriseMeta.NormalizeNamespace()
		} else {
			src.EnterpriseMeta.MergeNoWildcard(&e.EnterpriseMeta)
			src.EnterpriseMeta.Normalize()
		}

		// Compute the precedence only AFTER normalizing namespaces since the
		// namespaces are factored into the calculation.
		src.Precedence = computeIntentionPrecedence(e, src)

		if legacyWrite {
			// We always force meta to be non-nil so that it's an empty map. This
			// makes it easy for API responses to not nil-check this everywhere.
			if src.LegacyMeta == nil {
				src.LegacyMeta = make(map[string]string)
			}
		} else {
			// Legacy fields are cleared, except LegacyMeta which we leave
			// populated so that we can later fail the write in Validate() and
			// give the user a warning about possible data loss.
			src.LegacyID = ""
			src.LegacyCreateTime = nil
			src.LegacyUpdateTime = nil
		}

		for _, perm := range src.Permissions {
			if perm.HTTP == nil {
				continue
			}

			for j := 0; j < len(perm.HTTP.Methods); j++ {
				perm.HTTP.Methods[j] = strings.ToUpper(perm.HTTP.Methods[j])
			}
		}
	}

	// The source intentions closer to the head of the list have higher
	// precedence. i.e. index 0 has the highest precedence.
	sort.SliceStable(e.Sources, func(i, j int) bool {
		return e.Sources[i].Precedence > e.Sources[j].Precedence
	})

	return nil
}

func timePointer(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// NOTE: this assumes that the namespaces have been fully normalized.
func computeIntentionPrecedence(entry *ServiceIntentionsConfigEntry, src *SourceIntention) int {
	// Max maintains the maximum value that the precedence can be depending
	// on the number of exact values in the destination.
	var max int
	switch intentionCountExact(entry.Name, &entry.EnterpriseMeta) {
	case 2:
		max = 9
	case 1:
		max = 6
	case 0:
		max = 3
	default:
		// This shouldn't be possible, just set it to zero
		return 0
	}
	// Given the maximum, the exact value is determined based on the
	// number of source exact values.
	countSrc := intentionCountExact(src.Name, &src.EnterpriseMeta)
	return max - (2 - countSrc)
}

// intentionCountExact counts the number of exact values (not wildcards) in
// the given namespace and name.
func intentionCountExact(name string, entMeta *acl.EnterpriseMeta) int {
	ns := entMeta.NamespaceOrDefault()

	// If NS is wildcard, pair must be */* since an exact service cannot follow a wildcard NS
	// */* is allowed, but */foo is not
	if ns == WildcardSpecifier {
		return 0
	}

	// only the namespace must be exact, since the */* case already returned.
	if name == WildcardSpecifier {
		return 1
	}

	return 2
}

func (e *ServiceIntentionsConfigEntry) Validate() error {
	return e.validate(false)
}

func (e *ServiceIntentionsConfigEntry) LegacyValidate() error {
	return e.validate(true)
}

func (e *ServiceIntentionsConfigEntry) HasWildcardDestination() bool {
	dstNS := e.EnterpriseMeta.NamespaceOrDefault()
	return dstNS == WildcardSpecifier || e.Name == WildcardSpecifier
}

func (e *ServiceIntentionsConfigEntry) HasAnyPermissions() bool {
	for _, src := range e.Sources {
		if len(src.Permissions) > 0 {
			return true
		}
	}
	return false
}

func (e *ServiceIntentionsConfigEntry) validate(legacyWrite bool) error {
	if e.Name == "" {
		return fmt.Errorf("Name is required")
	}

	if err := validateIntentionWildcards(e.Name, &e.EnterpriseMeta, "", ""); err != nil {
		return err
	}

	destIsWild := e.HasWildcardDestination()

	if e.JWT != nil {
		if err := e.JWT.Validate(); err != nil {
			return err
		}
	}

	if legacyWrite {
		if len(e.Meta) > 0 {
			return fmt.Errorf("Meta must be omitted for legacy intention writes")
		}
	} else {
		if err := validateConfigEntryMeta(e.Meta); err != nil {
			return err
		}
	}

	if len(e.Sources) == 0 {
		return fmt.Errorf("At least one source is required")
	}

	type qualifiedServiceName struct {
		ServiceName   ServiceName
		Peer          string
		SamenessGroup string
	}

	seenSources := make(map[qualifiedServiceName]struct{})
	for i, src := range e.Sources {
		if src.Name == "" {
			return fmt.Errorf("Sources[%d].Name is required", i)
		}

		if err := src.validateSamenessGroup(); err != nil {
			return fmt.Errorf("Sources[%d].SamenessGroup: %v ", i, err)
		}

		if err := validateIntentionWildcards(src.Name, &src.EnterpriseMeta, src.Peer, src.SamenessGroup); err != nil {
			return fmt.Errorf("Sources[%d].%v", i, err)
		}

		if err := validateSourceIntentionEnterpriseMeta(&src.EnterpriseMeta, &e.EnterpriseMeta); err != nil {
			return fmt.Errorf("Sources[%d].%v", i, err)
		}

		if src.Peer != "" && src.PartitionOrEmpty() != "" {
			return fmt.Errorf("Sources[%d].Peer: cannot set Peer and Partition at the same time.", i)
		}

		if src.SamenessGroup != "" && src.PartitionOrEmpty() != "" {
			return fmt.Errorf("Sources[%d].SamenessGroup: cannot set SamenessGroup and Partition at the same time", i)
		}

		if src.SamenessGroup != "" && src.Peer != "" {
			return fmt.Errorf("Sources[%d].SamenessGroup: cannot set SamenessGroup and Peer at the same time", i)
		}

		// Length of opaque values
		if len(src.Description) > metaValueMaxLength {
			return fmt.Errorf(
				"Sources[%d].Description exceeds maximum length %d", i, metaValueMaxLength)
		}

		if legacyWrite {
			if src.Peer != "" {
				return fmt.Errorf("Sources[%d].Peer cannot be set by legacy intentions", i)
			}

			if src.SamenessGroup != "" {
				return fmt.Errorf("Sources[%d].SamenessGroup cannot be set by legacy intentions", i)
			}

			if len(src.LegacyMeta) > metaMaxKeyPairs {
				return fmt.Errorf(
					"Sources[%d].Meta exceeds maximum element count %d", i, metaMaxKeyPairs)
			}
			for k, v := range src.LegacyMeta {
				if len(k) > metaKeyMaxLength {
					return fmt.Errorf(
						"Sources[%d].Meta key %q exceeds maximum length %d",
						i, k, metaKeyMaxLength,
					)
				}
				if len(v) > metaValueMaxLength {
					return fmt.Errorf(
						"Sources[%d].Meta value for key %q exceeds maximum length %d",
						i, k, metaValueMaxLength,
					)
				}
			}

			if src.LegacyCreateTime == nil {
				return fmt.Errorf("Sources[%d].LegacyCreateTime must be set", i)
			}
			if src.LegacyUpdateTime == nil {
				return fmt.Errorf("Sources[%d].LegacyUpdateTime must be set", i)
			}
		} else {
			if len(src.LegacyMeta) > 0 {
				return fmt.Errorf("Sources[%d].LegacyMeta must be omitted", i)
			}
			src.LegacyMeta = nil // ensure it's completely unset

			if src.LegacyCreateTime != nil {
				return fmt.Errorf("Sources[%d].LegacyCreateTime must be omitted", i)
			}
			if src.LegacyUpdateTime != nil {
				return fmt.Errorf("Sources[%d].LegacyUpdateTime must be omitted", i)
			}
		}

		if legacyWrite {
			if src.LegacyID == "" {
				return fmt.Errorf("Sources[%d].LegacyID must be set", i)
			}
		} else {
			if src.LegacyID != "" {
				return fmt.Errorf("Sources[%d].LegacyID must be omitted", i)
			}
		}

		if legacyWrite || len(src.Permissions) == 0 {
			switch src.Action {
			case IntentionActionAllow, IntentionActionDeny:
			default:
				return fmt.Errorf("Sources[%d].Action must be set to 'allow' or 'deny'", i)
			}
		}

		if len(src.Permissions) > 0 && src.Action != "" {
			return fmt.Errorf("Sources[%d].Action must be omitted if Permissions are specified", i)
		}

		if destIsWild && len(src.Permissions) > 0 {
			return fmt.Errorf("Sources[%d].Permissions cannot be specified on intentions with wildcarded destinations", i)
		}

		switch src.Type {
		case IntentionSourceConsul:
		default:
			return fmt.Errorf("Sources[%d].Type must be set to 'consul'", i)
		}

		for j, perm := range src.Permissions {
			switch perm.Action {
			case IntentionActionAllow, IntentionActionDeny:
			default:
				return fmt.Errorf("Sources[%d].Permissions[%d].Action must be set to 'allow' or 'deny'", i, j)
			}

			errorPrefix := "Sources[%d].Permissions[%d].HTTP"
			if perm.HTTP == nil {
				return fmt.Errorf(errorPrefix+" is required", i, j)
			}

			pathParts := 0
			if perm.HTTP.PathExact != "" {
				pathParts++
				if !strings.HasPrefix(perm.HTTP.PathExact, "/") {
					return fmt.Errorf(
						errorPrefix+".PathExact doesn't start with '/': %q",
						i, j, perm.HTTP.PathExact,
					)
				}
			}
			if perm.HTTP.PathPrefix != "" {
				pathParts++
				if !strings.HasPrefix(perm.HTTP.PathPrefix, "/") {
					return fmt.Errorf(
						errorPrefix+".PathPrefix doesn't start with '/': %q",
						i, j, perm.HTTP.PathPrefix,
					)
				}
			}
			if perm.HTTP.PathRegex != "" {
				pathParts++
			}
			if pathParts > 1 {
				return fmt.Errorf(
					errorPrefix+" should only contain at most one of PathExact, PathPrefix, or PathRegex",
					i, j,
				)
			}

			permParts := pathParts

			for k, hdr := range perm.HTTP.Header {
				if hdr.Name == "" {
					return fmt.Errorf(errorPrefix+".Header[%d] missing required Name field", i, j, k)
				}
				hdrParts := 0
				if hdr.Present {
					hdrParts++
				}
				if hdr.Exact != "" {
					hdrParts++
				}
				if hdr.Regex != "" {
					hdrParts++
				}
				if hdr.Prefix != "" {
					hdrParts++
				}
				if hdr.Suffix != "" {
					hdrParts++
				}
				if hdrParts != 1 {
					return fmt.Errorf(errorPrefix+".Header[%d] should only contain one of Present, Exact, Prefix, Suffix, or Regex", i, j, k)
				}
				permParts++
			}

			if len(perm.HTTP.Methods) > 0 {
				found := make(map[string]struct{})
				for _, m := range perm.HTTP.Methods {
					if !isValidHTTPMethod(m) {
						return fmt.Errorf(errorPrefix+".Methods contains an invalid method %q", i, j, m)
					}
					if _, ok := found[m]; ok {
						return fmt.Errorf(errorPrefix+".Methods contains %q more than once", i, j, m)
					}
					found[m] = struct{}{}
				}
				permParts++
			}

			if permParts == 0 {
				return fmt.Errorf(errorPrefix+" should not be empty", i, j)
			}

			if err := perm.Validate(); err != nil {
				return err
			}
		}

		qsn := qualifiedServiceName{Peer: src.Peer, SamenessGroup: src.SamenessGroup, ServiceName: src.SourceServiceName()}
		if _, exists := seenSources[qsn]; exists {
			if qsn.Peer != "" {
				return fmt.Errorf("Sources[%d] defines peer(%q) %q more than once", i, qsn.Peer, qsn.ServiceName.String())
			} else if qsn.SamenessGroup != "" {
				return fmt.Errorf("Sources[%d] defines sameness-group(%q) %q more than once", i, qsn.SamenessGroup, qsn.ServiceName.String())
			} else {
				return fmt.Errorf("Sources[%d] defines %q more than once", i, qsn.ServiceName.String())
			}
		}
		seenSources[qsn] = struct{}{}
	}

	return nil
}

// Wildcard usage verification
func validateIntentionWildcards(name string, entMeta *acl.EnterpriseMeta, peerName, samenessGroup string) error {
	ns := entMeta.NamespaceOrDefault()
	if ns != WildcardSpecifier {
		if strings.Contains(ns, WildcardSpecifier) {
			return fmt.Errorf("Namespace: wildcard character '*' cannot be used with partial values")
		}
	}
	if name != WildcardSpecifier {
		if strings.Contains(name, WildcardSpecifier) {
			return fmt.Errorf("Name: wildcard character '*' cannot be used with partial values")
		}

		if ns == WildcardSpecifier {
			return fmt.Errorf("Name: exact value cannot follow wildcard namespace")
		}
	}
	if strings.Contains(entMeta.PartitionOrDefault(), WildcardSpecifier) {
		return fmt.Errorf("Partition: cannot use wildcard '*' in partition")
	}
	if strings.Contains(peerName, WildcardSpecifier) {
		return fmt.Errorf("Peer: cannot use wildcard '*' in peer")
	}
	if strings.Contains(samenessGroup, WildcardSpecifier) {
		return fmt.Errorf("SamenessGroup: cannot use wildcard '*' in sameness group")
	}
	return nil
}

func (e *ServiceIntentionsConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *ServiceIntentionsConfigEntry) GetEnterpriseMeta() *acl.EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}

func (e *ServiceIntentionsConfigEntry) CanRead(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().IntentionReadAllowed(e.GetName(), &authzContext)
}

func (e *ServiceIntentionsConfigEntry) CanWrite(authz acl.Authorizer) error {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.ToAllowAuthorizer().IntentionWriteAllowed(e.GetName(), &authzContext)
}

func MigrateIntentions(ixns Intentions) []*ServiceIntentionsConfigEntry {
	if len(ixns) == 0 {
		return nil
	}
	collated := make(map[ServiceName]*ServiceIntentionsConfigEntry)
	for _, ixn := range ixns {
		thisEntry := ixn.ToConfigEntry(true)
		sn := thisEntry.DestinationServiceName()

		if entry, ok := collated[sn]; ok {
			entry.Sources = append(entry.Sources, thisEntry.Sources...)
		} else {
			collated[sn] = thisEntry
		}
	}

	out := make([]*ServiceIntentionsConfigEntry, 0, len(collated))
	for _, entry := range collated {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		a := out[i]
		b := out[j]

		if a.PartitionOrDefault() < b.PartitionOrDefault() {
			return true
		} else if a.PartitionOrDefault() > b.PartitionOrDefault() {
			return false
		}

		if a.NamespaceOrDefault() < b.NamespaceOrDefault() {
			return true
		} else if a.NamespaceOrDefault() > b.NamespaceOrDefault() {
			return false
		}

		return a.Name < b.Name
	})
	return out
}
