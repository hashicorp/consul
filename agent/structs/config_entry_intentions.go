package structs

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/consul/acl"
)

type ServiceIntentionsConfigEntry struct {
	Kind string
	Name string // formerly DestinationName

	Sources []*SourceIntention

	Meta map[string]string `json:",omitempty"` // formerly Intention.Meta

	EnterpriseMeta `hcl:",squash" mapstructure:",squash"` // formerly DestinationNS
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

	return &e2
}

func (e *ServiceIntentionsConfigEntry) DestinationServiceName() ServiceName {
	return NewServiceName(e.Name, &e.EnterpriseMeta)
}

func (e *ServiceIntentionsConfigEntry) ToIntention(src *SourceIntention) *Intention {
	meta := e.Meta
	if src.LegacyID != "" {
		meta = src.LegacyMeta
	}

	ixn := &Intention{
		ID:              src.LegacyID,
		Description:     src.Description,
		SourceNS:        src.NamespaceOrDefault(),
		SourceName:      src.Name,
		SourceType:      src.Type,
		Action:          src.Action,
		Meta:            meta,
		Precedence:      src.Precedence,
		DestinationNS:   e.NamespaceOrDefault(),
		DestinationName: e.Name,
		RaftIndex:       e.RaftIndex,
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
	Action IntentionAction

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
	EnterpriseMeta `hcl:",squash" mapstructure:",squash"`
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
		prevSourceByName     = make(map[ServiceName]*SourceIntention)
		prevSourceByLegacyID = make(map[string]*SourceIntention)
	)
	for _, src := range prev.Sources {
		prevSourceByName[src.SourceServiceName()] = src
		if src.LegacyID != "" {
			prevSourceByLegacyID[src.LegacyID] = src
		}
	}

	for i, src := range e.Sources {
		if src.LegacyID == "" {
			continue
		}

		// Check that the LegacyID fields are handled correctly during updates.
		if prevSrc, ok := prevSourceByName[src.SourceServiceName()]; ok {
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

	e.Kind = ServiceIntentions

	e.EnterpriseMeta.Normalize()

	for _, src := range e.Sources {
		// Default source type
		if src.Type == "" {
			src.Type = IntentionSourceConsul
		}

		// If the source namespace is omitted it inherits that of the
		// destination.
		src.EnterpriseMeta.MergeNoWildcard(&e.EnterpriseMeta)
		src.EnterpriseMeta.Normalize()

		// Compute the precedence only AFTER normalizing namespaces since the
		// namespaces are factored into the calculation.
		src.Precedence = computeIntentionPrecedence(e, src)

		if legacyWrite {
			// We always force meta to be non-nil so that it's an empty map. This
			// makes it easy for API responses to not nil-check this everywhere.
			if src.LegacyMeta == nil {
				src.LegacyMeta = make(map[string]string)
			}
			// Set the created/updated times. If this is an update instead of an insert
			// the UpdateOver() will fix it up appropriately.
			now := time.Now().UTC()
			src.LegacyCreateTime = timePointer(now)
			src.LegacyUpdateTime = timePointer(now)
		} else {
			// Legacy fields are cleared, except LegacyMeta which we leave
			// populated so that we can later fail the write in Validate() and
			// give the user a warning about possible data loss.
			src.LegacyID = ""
			src.LegacyCreateTime = nil
			src.LegacyUpdateTime = nil
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
func intentionCountExact(name string, entMeta *EnterpriseMeta) int {
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

func (e *ServiceIntentionsConfigEntry) validate(legacyWrite bool) error {
	if e.Name == "" {
		return fmt.Errorf("Name is required")
	}

	if err := validateIntentionWildcards(e.Name, &e.EnterpriseMeta); err != nil {
		return err
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

	seenSources := make(map[ServiceName]struct{})
	for i, src := range e.Sources {
		if src.Name == "" {
			return fmt.Errorf("Sources[%d].Name is required", i)
		}

		if err := validateIntentionWildcards(src.Name, &src.EnterpriseMeta); err != nil {
			return fmt.Errorf("Sources[%d].%v", i, err)
		}

		// Length of opaque values
		if len(src.Description) > metaValueMaxLength {
			return fmt.Errorf(
				"Sources[%d].Description exceeds maximum length %d", i, metaValueMaxLength)
		}

		if legacyWrite {
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
		} else {
			if len(src.LegacyMeta) > 0 {
				return fmt.Errorf("Sources[%d].LegacyMeta must be omitted", i)
			}
			src.LegacyMeta = nil // ensure it's completely unset
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

		switch src.Action {
		case IntentionActionAllow, IntentionActionDeny:
		default:
			return fmt.Errorf("Sources[%d].Action must be set to 'allow' or 'deny'", i)
		}

		switch src.Type {
		case IntentionSourceConsul:
		default:
			return fmt.Errorf("Sources[%d].Type must be set to 'consul'", i)
		}

		serviceName := src.SourceServiceName()
		if _, exists := seenSources[serviceName]; exists {
			return fmt.Errorf("Sources[%d] defines %q more than once", i, serviceName.String())
		}
		seenSources[serviceName] = struct{}{}
	}

	return nil
}

// Wildcard usage verification
func validateIntentionWildcards(name string, entMeta *EnterpriseMeta) error {
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
	return nil
}

func (e *ServiceIntentionsConfigEntry) GetRaftIndex() *RaftIndex {
	if e == nil {
		return &RaftIndex{}
	}

	return &e.RaftIndex
}

func (e *ServiceIntentionsConfigEntry) GetEnterpriseMeta() *EnterpriseMeta {
	if e == nil {
		return nil
	}

	return &e.EnterpriseMeta
}

func (e *ServiceIntentionsConfigEntry) CanRead(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.IntentionRead(e.GetName(), &authzContext) == acl.Allow
}

func (e *ServiceIntentionsConfigEntry) CanWrite(authz acl.Authorizer) bool {
	var authzContext acl.AuthorizerContext
	e.FillAuthzContext(&authzContext)
	return authz.IntentionWrite(e.GetName(), &authzContext) == acl.Allow
}

func MigrateIntentions(ixns Intentions) []*ServiceIntentionsConfigEntry {
	if len(ixns) == 0 {
		return nil
	}
	collated := make(map[ServiceName]*ServiceIntentionsConfigEntry)
	for _, ixn := range ixns {
		thisEntry := ixn.ToConfigEntry()
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
	return out
}
