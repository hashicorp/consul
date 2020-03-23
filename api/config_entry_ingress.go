package api

// IngressGatewayConfigEntry manages the configuration for an ingress service
// with the given name.
type IngressGatewayConfigEntry struct {
	Kind string
	Name string

	Listeners []IngressListener

	CreateIndex uint64
	ModifyIndex uint64
}

type IngressListener struct {
	Port     int
	Protocol string
	Header   string

	Services []IngressService
}

type IngressService struct {
	Name          string
	Namespace     string
	ServiceSubset string
}

func (i *IngressGatewayConfigEntry) GetKind() string {
	return i.Kind
}

func (i *IngressGatewayConfigEntry) GetName() string {
	return i.Name
}

func (i *IngressGatewayConfigEntry) GetCreateIndex() uint64 {
	return i.CreateIndex
}

func (i *IngressGatewayConfigEntry) GetModifyIndex() uint64 {
	return i.ModifyIndex
}
