package discoverychain

import "github.com/hashicorp/consul/agent/structs"

func synthesizeTCPRouteDiscoveryChain(route structs.TCPRouteConfigEntry) []structs.IngressService {
	services := []structs.IngressService{}
	for _, service := range route.Services {
		ingress := structs.IngressService{
			Name:           service.Name,
			EnterpriseMeta: service.EnterpriseMeta,
		}

		services = append(services, ingress)
	}

	return services
}
