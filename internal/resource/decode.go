package resource

import (
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

// DecodedResource is a generic holder to contain an original Resource and its
// decoded contents.
type DecodedResource[V any, PV interface {
	proto.Message
	*V
}] struct {
	Resource *pbresource.Resource
	Data     PV
}

// Decode will generically decode the provided resource into a 2-field
// structure that holds onto the original Resource and the decoded contents.
//
// Returns an ErrDataParse on unmarshalling errors.
func Decode[V any, PV interface {
	proto.Message
	*V
}](res *pbresource.Resource) (*DecodedResource[V, PV], error) {
	data := PV(new(V))
	if err := res.Data.UnmarshalTo(data); err != nil {
		return nil, NewErrDataParse(data, err)
	}
	return &DecodedResource[V, PV]{
		Resource: res,
		Data:     data,
	}, nil
}
