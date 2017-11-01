package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ScalewayContainerData represents a Scaleway container data (S3)
type ScalewayContainerData struct {
	LastModified string `json:"last_modified"`
	Name         string `json:"name"`
	Size         string `json:"size"`
}

// ScalewayGetContainerDatas represents a list of Scaleway containers data (S3)
type ScalewayGetContainerDatas struct {
	Container []ScalewayContainerData `json:"container"`
}

// ScalewayContainer represents a Scaleway container (S3)
type ScalewayContainer struct {
	ScalewayOrganizationDefinition `json:"organization"`
	Name                           string `json:"name"`
	Size                           string `json:"size"`
}

// ScalewayGetContainers represents a list of Scaleway containers (S3)
type ScalewayGetContainers struct {
	Containers []ScalewayContainer `json:"containers"`
}

// GetContainers returns a ScalewayGetContainers
func (s *ScalewayAPI) GetContainers() (*ScalewayGetContainers, error) {
	resp, err := s.GetResponsePaginate(s.computeAPI, "containers", url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var containers ScalewayGetContainers

	if err = json.Unmarshal(body, &containers); err != nil {
		return nil, err
	}
	return &containers, nil
}

// GetContainerDatas returns a ScalewayGetContainerDatas
func (s *ScalewayAPI) GetContainerDatas(container string) (*ScalewayGetContainerDatas, error) {
	resp, err := s.GetResponsePaginate(s.computeAPI, fmt.Sprintf("containers/%s", container), url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var datas ScalewayGetContainerDatas

	if err = json.Unmarshal(body, &datas); err != nil {
		return nil, err
	}
	return &datas, nil
}
