package pbconnect

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbcommon"
	"github.com/hashicorp/consul/proto/pbutil"
)

func CARootsToStructs(s *CARoots) (*structs.IndexedCARoots, error) {
	if s == nil {
		return nil, nil
	}
	var t structs.IndexedCARoots
	t.ActiveRootID = s.ActiveRootID
	t.TrustDomain = s.TrustDomain
	t.Roots = make([]*structs.CARoot, len(s.Roots))
	for i := range s.Roots {
		root, err := CARootToStructs(s.Roots[i])
		if err != nil {
			return &t, err
		}
		t.Roots[i] = root
	}
	queryMeta, err := pbcommon.QueryMetaToStructs(s.QueryMeta)
	if err != nil {
		return &t, nil
	}
	t.QueryMeta = queryMeta
	return &t, nil
}

func NewCARootsFromStructs(s *structs.IndexedCARoots) (*CARoots, error) {
	if s == nil {
		return nil, nil
	}
	var t CARoots
	t.ActiveRootID = s.ActiveRootID
	t.TrustDomain = s.TrustDomain
	t.Roots = make([]*CARoot, len(s.Roots))
	for i := range s.Roots {
		root, err := NewCARootFromStructs(s.Roots[i])
		if err != nil {
			return &t, err
		}
		t.Roots[i] = root
	}
	queryMeta, err := pbcommon.NewQueryMetaFromStructs(s.QueryMeta)
	if err != nil {
		return &t, nil
	}
	t.QueryMeta = queryMeta
	return &t, nil
}

func CARootToStructs(s *CARoot) (*structs.CARoot, error) {
	if s == nil {
		return nil, nil
	}
	var t structs.CARoot
	t.ID = s.ID
	t.Name = s.Name
	t.SerialNumber = s.SerialNumber
	t.SigningKeyID = s.SigningKeyID
	t.ExternalTrustDomain = s.ExternalTrustDomain
	notBefore, err := pbutil.TimeFromProto(s.NotBefore)
	if err != nil {
		return &t, nil
	}
	t.NotBefore = notBefore
	notAfter, err := pbutil.TimeFromProto(s.NotAfter)
	if err != nil {
		return &t, nil
	}
	t.NotAfter = notAfter
	t.RootCert = s.RootCert
	if len(s.IntermediateCerts) > 0 {
		t.IntermediateCerts = make([]string, len(s.IntermediateCerts))
		copy(t.IntermediateCerts, s.IntermediateCerts)
	}
	t.SigningCert = s.SigningCert
	t.SigningKey = s.SigningKey
	t.Active = s.Active
	rotatedOutAt, err := pbutil.TimeFromProto(s.RotatedOutAt)
	if err != nil {
		return &t, nil
	}
	t.RotatedOutAt = rotatedOutAt
	t.PrivateKeyType = s.PrivateKeyType
	t.PrivateKeyBits = int(s.PrivateKeyBits)
	t.RaftIndex = pbcommon.RaftIndexToStructs(s.RaftIndex)
	return &t, nil
}

func NewCARootFromStructs(s *structs.CARoot) (*CARoot, error) {
	if s == nil {
		return nil, nil
	}
	var t CARoot
	t.ID = s.ID
	t.Name = s.Name
	t.SerialNumber = s.SerialNumber
	t.SigningKeyID = s.SigningKeyID
	t.ExternalTrustDomain = s.ExternalTrustDomain
	notBefore, err := pbutil.TimeToProto(s.NotBefore)
	if err != nil {
		return &t, err
	}
	t.NotBefore = notBefore
	notAfter, err := pbutil.TimeToProto(s.NotAfter)
	if err != nil {
		return &t, err
	}
	t.NotAfter = notAfter
	t.RootCert = s.RootCert
	if len(s.IntermediateCerts) > 0 {
		t.IntermediateCerts = make([]string, len(s.IntermediateCerts))
		copy(t.IntermediateCerts, s.IntermediateCerts)
	}
	t.SigningCert = s.SigningCert
	t.SigningKey = s.SigningKey
	t.Active = s.Active
	rotatedOutAt, err := pbutil.TimeToProto(s.RotatedOutAt)
	if err != nil {
		return &t, err
	}
	t.RotatedOutAt = rotatedOutAt
	t.PrivateKeyType = s.PrivateKeyType
	t.PrivateKeyBits = int32(s.PrivateKeyBits)
	t.RaftIndex = pbcommon.NewRaftIndexFromStructs(s.RaftIndex)
	return &t, nil
}

func IssuedCertToStructs(s *IssuedCert) (*structs.IssuedCert, error) {
	if s == nil {
		return nil, nil
	}
	var t structs.IssuedCert
	t.SerialNumber = s.SerialNumber
	t.CertPEM = s.CertPEM
	t.PrivateKeyPEM = s.PrivateKeyPEM
	t.Service = s.Service
	t.ServiceURI = s.ServiceURI
	t.Agent = s.Agent
	t.AgentURI = s.AgentURI
	validAfter, err := pbutil.TimeFromProto(s.ValidAfter)
	if err != nil {
		return &t, err
	}
	t.ValidAfter = validAfter
	validBefore, err := pbutil.TimeFromProto(s.ValidBefore)
	if err != nil {
		return &t, err
	}
	t.ValidBefore = validBefore
	t.EnterpriseMeta = pbcommon.EnterpriseMetaToStructs(s.EnterpriseMeta)
	t.RaftIndex = pbcommon.RaftIndexToStructs(s.RaftIndex)
	return &t, nil
}

func NewIssuedCertFromStructs(s *structs.IssuedCert) (*IssuedCert, error) {
	if s == nil {
		return nil, nil
	}
	var t IssuedCert
	t.SerialNumber = s.SerialNumber
	t.CertPEM = s.CertPEM
	t.PrivateKeyPEM = s.PrivateKeyPEM
	t.Service = s.Service
	t.ServiceURI = s.ServiceURI
	t.Agent = s.Agent
	t.AgentURI = s.AgentURI
	validAfter, err := pbutil.TimeToProto(s.ValidAfter)
	if err != nil {
		return &t, err
	}
	t.ValidAfter = validAfter
	validBefore, err := pbutil.TimeToProto(s.ValidBefore)
	if err != nil {
		return &t, err
	}
	t.ValidBefore = validBefore
	t.EnterpriseMeta = pbcommon.NewEnterpriseMetaFromStructs(s.EnterpriseMeta)
	t.RaftIndex = pbcommon.NewRaftIndexFromStructs(s.RaftIndex)
	return &t, nil
}
