//go:build !consulent
// +build !consulent

package structs

func (us *Upstream) GetEnterpriseMeta() *EnterpriseMeta {
	return DefaultEnterpriseMetaInDefaultPartition()
}

func (us *Upstream) DestinationID() ServiceID {
	return ServiceID{
		ID: us.DestinationName,
	}
}

// Identifier returns a string representation that uniquely identifies the
// upstream in a canonical but human readable way.
func (us *Upstream) Identifier() string {
	name := us.DestinationName
	typ := us.DestinationType

	if typ != UpstreamDestTypePreparedQuery && us.DestinationNamespace != "" && us.DestinationNamespace != IntentionDefaultNamespace {
		name = us.DestinationNamespace + "/" + us.DestinationName
	}
	if us.Datacenter != "" {
		name += "?dc=" + us.Datacenter
	}

	// Service is default type so never prefix it. This is more readable and long
	// term it is the only type that matters so we can drop the prefix and have
	// nicer naming in metrics etc.
	if typ == "" || typ == UpstreamDestTypeService {
		return name
	}
	return typ + ":" + name
}
