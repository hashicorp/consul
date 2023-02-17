package structs

import (
	"fmt"
	"sort"
	"time"

	"golang.org/x/exp/slices"

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

func (s Status) SameConditions(other Status) bool {
	if len(s.Conditions) != len(other.Conditions) {
		return false
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

func lessResource(one, two *ResourceReference) bool {
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

// Condition is used for a single message and state associated
// with an object. For example, a ConfigEntry that references
// multiple other resources may have different statuses with
// respect to each of those resources.
type Condition struct {
	// Type is a value from a bounded set of condition types for a given controlled object
	Type string
	// Status is a value from a bounded set of statuses that an object might have
	Status ConditionStatus
	// Reason is a value from a bounded set of reasons for a given type
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

// ConditionStatus is a bounded set of statuses that an object might have
type ConditionStatus string

const (
	ConditionStatusTrue    ConditionStatus = "True"
	ConditionStatusFalse   ConditionStatus = "False"
	ConditionStatusUnknown ConditionStatus = "Unknown"
)

// GatewayConditionType is a type of condition for a route.
type GatewayConditionType string

// GatewayConditionReason is a reason for a route condition.
type GatewayConditionReason string

const (
	// This condition is true when the controller managing the Gateway is
	// syntactically and semantically valid enough to produce some configuration
	// in the underlying data plane. This does not indicate whether or not the
	// configuration has been propagated to the data plane.
	//
	// Possible reasons for this condition to be True are:
	//
	// * "Accepted"
	//
	// Possible reasons for this condition to be False are:
	//
	// * "Invalid"
	//
	// Possible reasons for this condition to be Unknown are:
	//
	// * "Pending"
	//
	GatewayConditionAccepted GatewayConditionType = "Accepted"

	// This reason is used with the "Accepted" condition when the condition is
	// True.
	GatewayReasonAccepted GatewayConditionReason = "Accepted"

	// This reason is used with the "Accepted" condition when the Gateway is
	// syntactically or semantically invalid.
	GatewayReasonInvalid GatewayConditionReason = "Invalid"

	// This reason is used with the "Accepted" and "ListenersConfigured"
	// conditions when the status is "Unknown" and no controller has reconciled
	// the Gateway.
	GatewayReasonPending GatewayConditionReason = "Pending"

	// This condition indicates that the controller was able to resolve
	// all specification requirements for all Gateway Listeners. If a Listener is
	// conflicted, its network port should not be configured on any network
	// elements. Even if a Listener is syntactically and semantically valid,
	// the controller may not able to configure it on the underlying Gateway
	// infrastructure. When setting this condition to False, a ResourceReference to
	// the misconfigured Listener should be provided.

	//
	// Possible reasons for this condition to be True are:
	//
	// * "ListenersConfigured"
	//
	// Possible reasons for this condition to be False are:
	//
	// * "ListenerHostnameConflict"
	// * "ListenerProtocolConflict"
	// * "ListenerPortUnavailable"
	// * "ListenerUnsupportedProtocol"
	// * "ListenerUnsupportedAddress"
	//
	// Possible reasons for this condition to be Unknown are:
	//
	// * "Pending"
	//
	GatewayConditionListenersConfigured GatewayConditionType = "ListenersConfigured"

	// This reason is used with the "ListenersConfigured" condition when the
	// condition is True.
	GatewayReasonListenersConfigured GatewayConditionReason = "ListenersConfigured"

	// This reason is used with the "ListenersConfigured" condition when
	// the Listener conflicts with hostnames in other Listeners. For
	// example, this reason would be used when multiple Listeners on
	// the same port use `example.com` in the hostname field.
	GatewayListenerReasonHostnameConflict GatewayConditionReason = "ListenerHostnameConflict"

	// This reason is used with the "ListenersConfigured" condition when
	// multiple Listeners are specified with the same Listener port
	// number, but have conflicting protocol specifications.
	GatewayListenerReasonProtocolConflict GatewayConditionReason = "ListenerProtocolConflict"

	// This reason is used with the "ListenersConfigured" condition when the Listener
	// requests a port that cannot be used on the Gateway. This reason could be
	// used in a number of instances, including:
	//
	// * The port is already in use.
	// * The port is not supported by the implementation.
	GatewayListenerReasonPortUnavailable GatewayConditionReason = "ListenerPortUnavailable"

	// This reason is used with the "ListenersConfigured" condition when the
	// Listener could not be attached to be Gateway because its
	// protocol type is not supported.
	GatewayListenerReasonUnsupportedProtocol GatewayConditionReason = "ListenerUnsupportedProtocol"

	// This reason is used with the "ListenersConfigured" condition when a
	// Listener could not be attached to the Gateway because the requested address
	// is not supported. This reason could be used in a number of instances,
	// including:
	//
	// * The address is already in use.
	// * The type of address is not supported by the implementation.
	GatewayListenerReasonUnsupportedAddress GatewayConditionReason = "ListenerUnsupportedAddress"

	// This condition indicates whether the controller was able to
	// resolve all the object references for the Gateway. When setting this
	// condition to False, a ResourceReference to the misconfigured Listener should
	// be provided.
	//
	// Possible reasons for this condition to be true are:
	//
	// * "ResolvedRefs"
	//
	// Possible reasons for this condition to be False are:
	//
	// * "InvalidCertificateRef"
	// * "InvalidRouteKinds"
	// * "RefNotPermitted"
	//
	GatewayConditionResolvedRefs GatewayConditionType = "ResolvedRefs"

	// This reason is used with the "ResolvedRefs" condition when the condition
	// is true.
	GatewayReasonResolvedRefs GatewayConditionReason = "ResolvedRefs"

	// This reason is used with the "ResolvedRefs" condition when a
	// Listener has a TLS configuration with at least one TLS CertificateRef
	// that is invalid or does not exist.
	// A CertificateRef is considered invalid when it refers to a nonexistent
	// or unsupported resource or kind, or when the data within that resource
	// is malformed.
	// This reason must be used only when the reference is allowed, either by
	// referencing an object in the same namespace as the Gateway, or when
	// a cross-namespace reference has been explicitly allowed by a ReferenceGrant.
	// If the reference is not allowed, the reason RefNotPermitted must be used
	// instead.
	GatewayListenerReasonInvalidCertificateRef GatewayConditionReason = "InvalidCertificateRef"

	// This reason is used with the "ResolvedRefs" condition when an invalid or
	// unsupported Route kind is specified by a Listener.
	GatewayListenerReasonInvalidRouteKinds GatewayConditionReason = "InvalidRouteKinds"

	// This reason is used with the "ResolvedRefs" condition when a
	// Listener has a TLS configuration that references an object in another
	// namespace, where the object in the other namespace does not have a
	// ReferenceGrant explicitly allowing the reference.
	GatewayListenerReasonRefNotPermitted GatewayConditionReason = "RefNotPermitted"
)

// RouteConditionType is a type of condition for a route.
type RouteConditionType string

// RouteConditionReason is a reason for a route condition.
type RouteConditionReason string

const (
	// This condition indicates whether the route has been accepted or rejected
	// by a Gateway, and why.
	//
	// Possible reasons for this condition to be true are:
	//
	// * "Accepted"
	//
	// Possible reasons for this condition to be False are:
	//
	// * "NotAllowedByListeners"
	// * "NoMatchingListenerHostname"
	// * "NoMatchingParent"
	// * "UnsupportedValue"
	// * "ParentRefNotPermitted"
	//
	// Possible reasons for this condition to be Unknown are:
	//
	// * "Pending"
	//
	// Controllers may raise this condition with other reasons,
	// but should prefer to use the reasons listed above to improve
	// interoperability.
	RouteConditionAccepted RouteConditionType = "Accepted"

	// This reason is used with the "Accepted" condition when the Route has been
	// accepted by the Gateway.
	RouteReasonAccepted RouteConditionReason = "Accepted"

	// This reason is used with the "Accepted" condition when the route has not
	// been accepted by a Gateway because the Gateway has no Listener whose
	// allowedRoutes criteria permit the route
	RouteReasonNotAllowedByListeners RouteConditionReason = "NotAllowedByListeners"

	// This reason is used with the "Accepted" condition when the Gateway has no
	// compatible Listeners whose Hostname matches the route
	RouteReasonNoMatchingListenerHostname RouteConditionReason = "NoMatchingListenerHostname"

	// This reason is used with the "Accepted" condition when there are
	// no matching Parents. In the case of Gateways, this can occur when
	// a Route ParentRef specifies a Port and/or SectionName that does not
	// match any Listeners in the Gateway.
	RouteReasonNoMatchingParent RouteConditionReason = "NoMatchingParent"

	// This reason is used with the "Accepted" condition when a value for an Enum
	// is not recognized.
	RouteReasonUnsupportedValue RouteConditionReason = "UnsupportedValue"

	// This reason is used with the "Accepted" condition when the route has not
	// been accepted by a Gateway because it has a cross-namespace parentRef,
	// but no ReferenceGrant in the other namespace allows such a reference.
	RouteReasonParentRefNotPermitted RouteConditionReason = "ParentRefNotPermitted"

	// This reason is used with the "Accepted" when a controller has not yet
	// reconciled the route.
	RouteReasonPending RouteConditionReason = "Pending"

	// This condition indicates whether the controller was able to resolve all
	// the object references for the Route.
	//
	// Possible reasons for this condition to be true are:
	//
	// * "ResolvedRefs"
	//
	// Possible reasons for this condition to be false are:
	//
	// * "RefNotPermitted"
	// * "InvalidKind"
	// * "BackendNotFound"
	//
	// Controllers may raise this condition with other reasons,
	// but should prefer to use the reasons listed above to improve
	// interoperability.
	RouteConditionResolvedRefs RouteConditionType = "ResolvedRefs"

	// This reason is used with the "ResolvedRefs" condition when the condition
	// is true.
	RouteReasonResolvedRefs RouteConditionReason = "ResolvedRefs"

	// This reason is used with the "ResolvedRefs" condition when
	// one of the Listener's Routes has a BackendRef to an object in
	// another namespace, where the object in the other namespace does
	// not have a ReferenceGrant explicitly allowing the reference.
	RouteReasonRefNotPermitted RouteConditionReason = "RefNotPermitted"

	// This reason is used with the "ResolvedRefs" condition when
	// one of the Route's rules has a reference to an unknown or unsupported
	// Group and/or Kind.
	RouteReasonInvalidKind RouteConditionReason = "InvalidKind"

	// This reason is used with the "ResolvedRefs" condition when one of the
	// Route's rules has a reference to a resource that does not exist.
	RouteReasonBackendNotFound RouteConditionReason = "BackendNotFound"
)

func (c *Condition) IsCondition(other *Condition) bool {
	return c.Type == other.Type && c.Resource.IsSame(other.Resource)
}

func (c *Condition) IsSame(other *Condition) bool {
	return c.IsCondition(other) &&
		c.Type == other.Type &&
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

// NewGatewayCondition is a helper to build allowable Conditions for a Route config entry
func NewGatewayCondition(name GatewayConditionType, status ConditionStatus, reason GatewayConditionReason, message string) Condition {
	if err := checkGatewayConditionReason(name, status, reason); err != nil {
		panic(err)
	}

	return Condition{
		Type:               string(name),
		Status:             status,
		Reason:             string(reason),
		Message:            message,
		LastTransitionTime: pointerTo(time.Now().UTC()),
	}
}

func checkGatewayConditionReason(name GatewayConditionType, status ConditionStatus, reason GatewayConditionReason) error {
	if err := checkConditionStatus(status); err != nil {
		return err
	}

	gatewayConditionReasons := map[GatewayConditionType]map[ConditionStatus][]GatewayConditionReason{
		GatewayConditionAccepted: {
			ConditionStatusTrue: {
				GatewayReasonAccepted,
			},
			ConditionStatusFalse: {
				GatewayReasonInvalid,
			},
			ConditionStatusUnknown: {
				GatewayReasonPending,
			},
		},
		GatewayConditionListenersConfigured: {
			ConditionStatusTrue: {
				GatewayReasonListenersConfigured,
			},
			ConditionStatusFalse: {
				GatewayListenerReasonHostnameConflict,
				GatewayListenerReasonProtocolConflict,
				GatewayListenerReasonPortUnavailable,
				GatewayListenerReasonUnsupportedProtocol,
				GatewayListenerReasonUnsupportedAddress,
			},
			ConditionStatusUnknown: {
				GatewayReasonPending,
			},
		},
		GatewayConditionResolvedRefs: {
			ConditionStatusTrue: {
				GatewayReasonResolvedRefs,
			},
			ConditionStatusFalse: {
				GatewayListenerReasonInvalidCertificateRef,
				GatewayListenerReasonInvalidRouteKinds,
				GatewayListenerReasonRefNotPermitted,
			},
			ConditionStatusUnknown: {},
		},
	}

	reasons, ok := gatewayConditionReasons[name]
	if !ok {
		return fmt.Errorf("unrecognized GatewayConditionType %s", name)
	}

	reasonsForStatus, ok := reasons[status]
	if !ok {
		return fmt.Errorf("unrecognized ConditionStatus %s", name)
	}

	if !slices.Contains(reasonsForStatus, reason) {
		return fmt.Errorf("gateway condition reason %s not allowed for gateway condition type %s with status %s", reason, name, status)
	}

	return nil
}

// NewRouteCondition is a helper to build allowable Conditions for a Route config entry
func NewRouteCondition(name RouteConditionType, status ConditionStatus, reason RouteConditionReason, message string) Condition {
	if err := checkRouteConditionReason(name, status, reason); err != nil {
		panic(err)
	}

	return Condition{
		Type:               string(name),
		Status:             status,
		Reason:             string(reason),
		Message:            message,
		LastTransitionTime: pointerTo(time.Now().UTC()),
	}
}

func checkRouteConditionReason(name RouteConditionType, status ConditionStatus, reason RouteConditionReason) error {
	if err := checkConditionStatus(status); err != nil {
		return err
	}

	routeConditionReasons := map[RouteConditionType]map[ConditionStatus][]RouteConditionReason{
		RouteConditionAccepted: {
			ConditionStatusTrue: {
				RouteReasonAccepted,
			},
			ConditionStatusFalse: {
				RouteReasonNotAllowedByListeners,
				RouteReasonNoMatchingListenerHostname,
				RouteReasonNoMatchingParent,
				RouteReasonUnsupportedValue,
				RouteReasonParentRefNotPermitted,
			},
			ConditionStatusUnknown: {
				RouteReasonPending,
			},
		},
		RouteConditionResolvedRefs: {
			ConditionStatusTrue: {
				RouteReasonResolvedRefs,
			},
			ConditionStatusFalse: {
				RouteReasonRefNotPermitted,
				RouteReasonInvalidKind,
				RouteReasonBackendNotFound,
			},
			ConditionStatusUnknown: {},
		},
	}

	reasons, ok := routeConditionReasons[name]
	if !ok {
		return fmt.Errorf("unrecognized RouteConditionType %s", name)
	}

	reasonsForStatus, ok := reasons[status]
	if !ok {
		return fmt.Errorf("unrecognized ConditionStatus %s", name)
	}

	if !slices.Contains(reasonsForStatus, reason) {
		return fmt.Errorf("route condition reason %s not allowed for route condition type %s with status %s", reason, name, status)
	}

	return nil
}

func checkConditionStatus(status ConditionStatus) error {
	switch status {
	case ConditionStatusTrue:
	case ConditionStatusFalse:
	case ConditionStatusUnknown:
		return nil
	default:
		return fmt.Errorf("unrecognized ConditionStatus %s", status)
	}

	// FIXME: this will never be reached, can it be removed somehow?
	return nil
}

// FIXME: duplicated from agent/consul/gateways/controller_gateways.go, centralize helper somewhere?
// pointerTo returns a pointer to the value passed as an argument
func pointerTo[T any](value T) *T {
	return &value
}
