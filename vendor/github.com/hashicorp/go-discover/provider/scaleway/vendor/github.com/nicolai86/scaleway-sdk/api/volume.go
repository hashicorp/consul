package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ScalewayVolume represents a Scaleway Volume
type ScalewayVolume struct {
	// Identifier is a unique identifier for the volume
	Identifier string `json:"id,omitempty"`

	// Size is the allocated size of the volume
	Size uint64 `json:"size,omitempty"`

	// CreationDate is the creation date of the volume
	CreationDate string `json:"creation_date,omitempty"`

	// ModificationDate is the date of the last modification of the volume
	ModificationDate string `json:"modification_date,omitempty"`

	// Organization is the organization owning the volume
	Organization string `json:"organization,omitempty"`

	// Name is the name of the volume
	Name string `json:"name,omitempty"`

	// Server is the server using this image
	Server *struct {
		Identifier string `json:"id,omitempty"`
		Name       string `json:"name,omitempty"`
	} `json:"server,omitempty"`

	// VolumeType is a Scaleway identifier for the kind of volume (default: l_ssd)
	VolumeType string `json:"volume_type,omitempty"`

	// ExportURI represents the url used by initrd/scripts to attach the volume
	ExportURI string `json:"export_uri,omitempty"`
}

type volumeResponse struct {
	Volume ScalewayVolume `json:"volume,omitempty"`
}

// ScalewayVolumeDefinition represents a Scaleway volume definition
type ScalewayVolumeDefinition struct {
	// Name is the user-defined name of the volume
	Name string `json:"name"`

	// Image is the image used by the volume
	Size uint64 `json:"size"`

	// Bootscript is the bootscript used by the volume
	Type string `json:"volume_type"`

	// Organization is the owner of the volume
	Organization string `json:"organization"`
}

// ScalewayVolumePutDefinition represents a Scaleway volume with nullable fields (for PUT)
type ScalewayVolumePutDefinition struct {
	Identifier       *string `json:"id,omitempty"`
	Size             *uint64 `json:"size,omitempty"`
	CreationDate     *string `json:"creation_date,omitempty"`
	ModificationDate *string `json:"modification_date,omitempty"`
	Organization     *string `json:"organization,omitempty"`
	Name             *string `json:"name,omitempty"`
	Server           struct {
		Identifier *string `json:"id,omitempty"`
		Name       *string `json:"name,omitempty"`
	} `json:"server,omitempty"`
	VolumeType *string `json:"volume_type,omitempty"`
	ExportURI  *string `json:"export_uri,omitempty"`
}

// PostVolume creates a new volume
func (s *ScalewayAPI) PostVolume(definition ScalewayVolumeDefinition) (string, error) {
	definition.Organization = s.Organization
	if definition.Type == "" {
		definition.Type = "l_ssd"
	}

	resp, err := s.PostResponse(s.computeAPI, "volumes", definition)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusCreated}, resp)
	if err != nil {
		return "", err
	}
	var volume volumeResponse

	if err = json.Unmarshal(body, &volume); err != nil {
		return "", err
	}
	return volume.Volume.Identifier, nil
}

// PutVolume updates a volume
func (s *ScalewayAPI) PutVolume(volumeID string, definition ScalewayVolumePutDefinition) error {
	resp, err := s.PutResponse(s.computeAPI, fmt.Sprintf("volumes/%s", volumeID), definition)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = s.handleHTTPError([]int{http.StatusOK}, resp)
	return err
}

// DeleteVolume deletes a volume
func (s *ScalewayAPI) DeleteVolume(volumeID string) error {
	resp, err := s.DeleteResponse(s.computeAPI, fmt.Sprintf("volumes/%s", volumeID))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err := s.handleHTTPError([]int{http.StatusNoContent}, resp); err != nil {
		return err
	}
	return nil
}

type volumesResponse struct {
	Volumes []ScalewayVolume `json:"volumes,omitempty"`
}

// GetVolumes gets the list of volumes from the ScalewayAPI
func (s *ScalewayAPI) GetVolumes() (*[]ScalewayVolume, error) {
	query := url.Values{}

	resp, err := s.GetResponsePaginate(s.computeAPI, "volumes", query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}

	var volumes volumesResponse

	if err = json.Unmarshal(body, &volumes); err != nil {
		return nil, err
	}
	return &volumes.Volumes, nil
}

// GetVolume gets a volume from the ScalewayAPI
func (s *ScalewayAPI) GetVolume(volumeID string) (*ScalewayVolume, error) {
	resp, err := s.GetResponsePaginate(s.computeAPI, "volumes/"+volumeID, url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var oneVolume volumeResponse

	if err = json.Unmarshal(body, &oneVolume); err != nil {
		return nil, err
	}
	// FIXME region, arch, owner, title
	return &oneVolume.Volume, nil
}
