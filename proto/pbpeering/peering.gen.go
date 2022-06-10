// Code generated by mog. DO NOT EDIT.

package pbpeering

import "github.com/hashicorp/consul/api"

func EstablishRequestToAPI(s *EstablishRequest, t *api.PeeringEstablishRequest) {
	if s == nil {
		return
	}
	t.PeerName = s.PeerName
	t.PeeringToken = s.PeeringToken
	t.Datacenter = s.Datacenter
	t.Token = s.Token
	t.Meta = s.Meta
}
func EstablishRequestFromAPI(t *api.PeeringEstablishRequest, s *EstablishRequest) {
	if s == nil {
		return
	}
	s.PeerName = t.PeerName
	s.PeeringToken = t.PeeringToken
	s.Datacenter = t.Datacenter
	s.Token = t.Token
	s.Meta = t.Meta
}
func EstablishResponseToAPI(s *EstablishResponse, t *api.PeeringEstablishResponse) {
	if s == nil {
		return
	}
}
func EstablishResponseFromAPI(t *api.PeeringEstablishResponse, s *EstablishResponse) {
	if s == nil {
		return
	}
}
func GenerateTokenRequestToAPI(s *GenerateTokenRequest, t *api.PeeringGenerateTokenRequest) {
	if s == nil {
		return
	}
	t.PeerName = s.PeerName
	t.Partition = s.Partition
	t.Datacenter = s.Datacenter
	t.Token = s.Token
	t.Meta = s.Meta
}
func GenerateTokenRequestFromAPI(t *api.PeeringGenerateTokenRequest, s *GenerateTokenRequest) {
	if s == nil {
		return
	}
	s.PeerName = t.PeerName
	s.Partition = t.Partition
	s.Datacenter = t.Datacenter
	s.Token = t.Token
	s.Meta = t.Meta
}
func GenerateTokenResponseToAPI(s *GenerateTokenResponse, t *api.PeeringGenerateTokenResponse) {
	if s == nil {
		return
	}
	t.PeeringToken = s.PeeringToken
}
func GenerateTokenResponseFromAPI(t *api.PeeringGenerateTokenResponse, s *GenerateTokenResponse) {
	if s == nil {
		return
	}
	s.PeeringToken = t.PeeringToken
}
func PeeringToAPI(s *Peering, t *api.Peering) {
	if s == nil {
		return
	}
	t.ID = s.ID
	t.Name = s.Name
	t.Partition = s.Partition
	t.Meta = s.Meta
	t.State = PeeringStateToAPI(s.State)
	t.PeerID = s.PeerID
	t.PeerCAPems = s.PeerCAPems
	t.PeerServerName = s.PeerServerName
	t.PeerServerAddresses = s.PeerServerAddresses
	t.CreateIndex = s.CreateIndex
	t.ModifyIndex = s.ModifyIndex
}
func PeeringFromAPI(t *api.Peering, s *Peering) {
	if s == nil {
		return
	}
	s.ID = t.ID
	s.Name = t.Name
	s.Partition = t.Partition
	s.Meta = t.Meta
	s.State = PeeringStateFromAPI(t.State)
	s.PeerID = t.PeerID
	s.PeerCAPems = t.PeerCAPems
	s.PeerServerName = t.PeerServerName
	s.PeerServerAddresses = t.PeerServerAddresses
	s.CreateIndex = t.CreateIndex
	s.ModifyIndex = t.ModifyIndex
}
