package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
)

type ServerAvailabilities map[string]interface{}

func (a ServerAvailabilities) CommercialTypes() []string {
	types := []string{}
	for k, v := range a {
		if _, ok := v.(bool); !ok {
			continue
		}
		types = append(types, k)
	}
	return types
}

func (s *ScalewayAPI) GetServerAvailabilities() (ServerAvailabilities, error) {
	resp, err := s.response("GET", fmt.Sprintf("%s/availability.json", s.availabilityAPI), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bs, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	content := ServerAvailabilities{}
	if err := json.Unmarshal(bs, &content); err != nil {
		return nil, err
	}
	return content, nil
}
