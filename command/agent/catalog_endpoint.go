package agent

import (
	"net/http"
)

func (s *HTTPServer) CatalogDatacenters(req *http.Request) (interface{}, error) {
	var out []string
	if err := s.agent.RPC("Catalog.ListDatacenters", struct{}{}, &out); err != nil {
		return nil, err
	}
	return out, nil
}
