//go:build !consulent
// +build !consulent

package acl

// In some sense we really want this to contain an EnterpriseMeta, but
// this turns out to be a convenient place to hang helper functions off of.
type EnterpriseObjectDescriptor struct {
	Name string
}

func MakeEnterpriseObjectDescriptor(name string, _ AuthorizerContext) EnterpriseObjectDescriptor {
	return EnterpriseObjectDescriptor{Name: name}
}

func (od *EnterpriseObjectDescriptor) ToString() string {
	return od.Name
}
