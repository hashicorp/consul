package resource

import "github.com/hashicorp/consul/proto-public/pbresource"

// EqualType compares two resource types for equality without reflection.
func EqualType(a, b *pbresource.Type) bool {
	if a == b {
		return true
	}

	return a.Group == b.Group &&
		a.GroupVersion == b.GroupVersion &&
		a.Kind == b.Kind
}

// EqualType compares two resource tenancies for equality without reflection.
func EqualTenancy(a, b *pbresource.Tenancy) bool {
	if a == b {
		return true
	}

	return a.Partition == b.Partition &&
		a.PeerName == b.PeerName &&
		a.Namespace == b.Namespace
}

// EqualType compares two resource IDs for equality without reflection.
func EqualID(a, b *pbresource.ID) bool {
	if a == b {
		return true
	}

	return EqualType(a.Type, b.Type) &&
		EqualTenancy(a.Tenancy, b.Tenancy) &&
		a.Name == b.Name &&
		a.Uid == b.Uid
}
