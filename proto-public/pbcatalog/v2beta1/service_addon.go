package catalogv2beta1

func (s *Service) IsMeshEnabled() bool {
	for _, port := range s.GetPorts() {
		if port.Protocol == Protocol_PROTOCOL_MESH {
			return true
		}
	}
	return false
}

func (s *Service) FindServicePort(name string) *ServicePort {
	for _, port := range s.GetPorts() {
		if port.TargetPort == name {
			return port
		}
	}
	return nil
}
