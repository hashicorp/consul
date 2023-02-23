package structs

import (
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/consul/acl"
)

// ResourceReference is a reference to a ConfigEntry
// with an optional reference to a subsection of that ConfigEntry
// that can be specified as SectionName
type ResourceReference struct {
	// Kind is the kind of ConfigEntry that this resource refers to.
	Kind string
	// Name is the identifier for the ConfigEntry this resource refers to.
	Name string
	// SectionName is a generic subresource identifier that specifies
	// a subset of the ConfigEntry to which this reference applies. Usage
	// of this field should be up to the controller that leverages it. If
	// unused, this should be blank.
	SectionName string

	acl.EnterpriseMeta
}

func (r *ResourceReference) String() string {
	return fmt.Sprintf("%s:%s/%s/%s/%s", r.Kind, r.PartitionOrDefault(), r.NamespaceOrDefault(), r.Name, r.SectionName)
}

func (r *ResourceReference) IsSame(other *ResourceReference) bool {
	if r == nil && other == nil {
		return true
	}
	if r == nil || other == nil {
		return false
	}
	return r.Kind == other.Kind &&
		r.Name == other.Name &&
		r.SectionName == other.SectionName &&
		r.EnterpriseMeta.IsSame(&other.EnterpriseMeta)
}

// Status is used for propagating back asynchronously calculated
// messages from control loops to a user
type Status struct {
	// Conditions is the set of condition objects associated with
	// a ConfigEntry status.
	Conditions []Condition
}

func (s *Status) MatchesConditionStatus(condition Condition) bool {
	for _, c := range s.Conditions {
		if c.IsCondition(&condition) &&
			c.Status == condition.Status {
			return true
		}
	}
	return false
}

func (s Status) SameConditions(other Status) bool {
	if len(s.Conditions) != len(other.Conditions) {
		return false
	}
	lessResource := func(one, two *ResourceReference) bool {
		if one == nil && two == nil {
			return false
		}
		if one == nil {
			return true
		}
		if two == nil {
			return false
		}
		if one.Kind < two.Kind {
			return true
		}
		if one.Kind > two.Kind {
			return false
		}
		if one.Name < two.Name {
			return true
		}
		if one.Name > two.Name {
			return false
		}
		return one.SectionName < two.SectionName
	}
	sortConditions := func(conditions []Condition) []Condition {
		sort.SliceStable(conditions, func(i, j int) bool {
			if conditions[i].Type < conditions[j].Type {
				return true
			}
			if conditions[i].Type > conditions[j].Type {
				return false
			}
			return lessResource(conditions[i].Resource, conditions[j].Resource)
		})
		return conditions
	}
	oneConditions := sortConditions(s.Conditions)
	twoConditions := sortConditions(other.Conditions)
	for i, condition := range oneConditions {
		other := twoConditions[i]
		if !condition.IsSame(&other) {
			return false
		}
	}
	return true
}

// Condition is used for a single message and state associated
// with an object. For example, a ConfigEntry that references
// multiple other resources may have different statuses with
// respect to each of those resources.
type Condition struct {
	// Type is a value from a bounded set of types that an object might have
	Type string
	// Status is a value from a bounded set of statuses that an object might have
	Status string
	// Reason is a value from a bounded set of reasons for a given status
	Reason string
	// Message is a message that gives more detailed information about
	// why a Condition has a given status and reason
	Message string
	// Resource is an optional reference to a resource for which this
	// condition applies
	Resource *ResourceReference
	// LastTransitionTime is the time at which this Condition was created
	LastTransitionTime *time.Time
}

func (c *Condition) IsCondition(other *Condition) bool {
	return c.Type == other.Type && c.Resource.IsSame(other.Resource)
}

func (c *Condition) IsSame(other *Condition) bool {
	return c.IsCondition(other) &&
		c.Status == other.Status &&
		c.Reason == other.Reason &&
		c.Message == other.Message
}

type StatusUpdater struct {
	entry  ControlledConfigEntry
	status Status
}

func NewStatusUpdater(entry ControlledConfigEntry) *StatusUpdater {
	status := entry.GetStatus()
	return &StatusUpdater{
		entry:  entry,
		status: *status.DeepCopy(),
	}
}

func (u *StatusUpdater) SetCondition(condition Condition) {
	for i, c := range u.status.Conditions {
		if c.IsCondition(&condition) {
			if !c.IsSame(&condition) {
				// the conditions aren't identical, merge this one in
				u.status.Conditions[i] = condition
			}
			// we either set the condition or it was already set, so
			// just return
			return
		}
	}
	u.status.Conditions = append(u.status.Conditions, condition)
}

func (u *StatusUpdater) ClearConditions() {
	u.status.Conditions = []Condition{}
}

func (u *StatusUpdater) RemoveCondition(condition Condition) {
	filtered := []Condition{}
	for _, c := range u.status.Conditions {
		if !c.IsCondition(&condition) {
			filtered = append(filtered, c)
		}
	}
	u.status.Conditions = filtered
}

func (u *StatusUpdater) UpdateEntry() (ControlledConfigEntry, bool) {
	if u.status.SameConditions(u.entry.GetStatus()) {
		return nil, false
	}
	u.entry.SetStatus(u.status)
	return u.entry, true
}
