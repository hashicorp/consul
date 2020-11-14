// +build !consulent

package structs

func (us *Upstream) GetEnterpriseMeta() *EnterpriseMeta {
	return DefaultEnterpriseMeta()
}

func (us *Upstream) DestinationID() ServiceID {
	return ServiceID{
		ID: us.DestinationName,
	}
}
