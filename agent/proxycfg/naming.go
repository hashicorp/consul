package proxycfg

import (
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

type UpstreamID struct {
	Type       string
	Name       string
	Datacenter string
	acl.EnterpriseMeta
}

func NewUpstreamID(u *structs.Upstream) UpstreamID {
	id := UpstreamID{
		Type:       u.DestinationType,
		Name:       u.DestinationName,
		Datacenter: u.Datacenter,
		EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(
			u.DestinationPartition,
			u.DestinationNamespace,
		),
	}
	id.normalize()
	return id
}

func NewUpstreamIDFromServiceName(sn structs.ServiceName) UpstreamID {
	id := UpstreamID{
		Name:           sn.Name,
		EnterpriseMeta: sn.EnterpriseMeta,
	}
	id.normalize()
	return id
}

func NewUpstreamIDFromServiceID(sid structs.ServiceID) UpstreamID {
	id := UpstreamID{
		Name:           sid.ID,
		EnterpriseMeta: sid.EnterpriseMeta,
	}
	id.normalize()
	return id
}

func NewUpstreamIDFromTargetID(tid string) UpstreamID {
	// Drop the leading subset if one is present in the target ID.
	separators := strings.Count(tid, ".")
	if separators > 3 {
		prefix := tid[:strings.Index(tid, ".")+1]
		tid = strings.TrimPrefix(tid, prefix)
	}

	split := strings.SplitN(tid, ".", 4)

	id := UpstreamID{
		Name:           split[0],
		EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(split[2], split[1]),
		Datacenter:     split[3],
	}
	id.normalize()
	return id
}

func (u *UpstreamID) normalize() {
	if u.Type == structs.UpstreamDestTypeService {
		u.Type = ""
	}

	u.EnterpriseMeta.Normalize()
}

// String encodes the UpstreamID into a string for use in agent cache keys.
// You can decode it back again using UpstreamIDFromString.
func (u UpstreamID) String() string {
	return UpstreamIDString(u.Type, u.Datacenter, u.Name, &u.EnterpriseMeta)
}

func (u UpstreamID) GoString() string {
	return u.String()
}

func UpstreamIDFromString(input string) UpstreamID {
	typ, dc, name, entMeta := ParseUpstreamIDString(input)
	id := UpstreamID{
		Type:           typ,
		Datacenter:     dc,
		Name:           name,
		EnterpriseMeta: *entMeta,
	}
	id.normalize()
	return id
}

const upstreamTypePreparedQueryPrefix = structs.UpstreamDestTypePreparedQuery + ":"

func ParseUpstreamIDString(input string) (typ, dc, name string, meta *acl.EnterpriseMeta) {
	if strings.HasPrefix(input, upstreamTypePreparedQueryPrefix) {
		typ = structs.UpstreamDestTypePreparedQuery
		input = strings.TrimPrefix(input, upstreamTypePreparedQueryPrefix)
	}

	idx := strings.LastIndex(input, "?dc=")
	if idx != -1 {
		dc = input[idx+4:]
		input = input[0:idx]
	}

	name, meta = parseInnerUpstreamIDString(input)

	return typ, dc, name, meta
}

// EnvoyID returns a string representation that uniquely identifies the
// upstream in a canonical but human readable way.
//
// This should be used for any situation where we generate identifiers in Envoy
// xDS structures for this upstream.
//
// This will ensure that generated identifiers for the same thing in OSS and
// Enterprise render the same and omit default namespaces and partitions.
func (u UpstreamID) EnvoyID() string {
	name := u.enterpriseIdentifierPrefix() + u.Name
	typ := u.Type

	if u.Datacenter != "" {
		name += "?dc=" + u.Datacenter
	}

	// Service is default type so never prefix it. This is more readable and long
	// term it is the only type that matters so we can drop the prefix and have
	// nicer naming in metrics etc.
	if typ == "" || typ == structs.UpstreamDestTypeService {
		return name
	}
	return typ + ":" + name
}

func UpstreamsToMap(us structs.Upstreams) map[UpstreamID]*structs.Upstream {
	upstreamMap := make(map[UpstreamID]*structs.Upstream)

	for i := range us {
		u := us[i]
		upstreamMap[NewUpstreamID(&u)] = &u
	}
	return upstreamMap
}
