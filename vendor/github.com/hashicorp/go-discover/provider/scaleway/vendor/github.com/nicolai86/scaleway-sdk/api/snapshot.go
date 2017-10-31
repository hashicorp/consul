package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ScalewaySnapshotDefinition represents a Scaleway snapshot definition
type ScalewaySnapshotDefinition struct {
	VolumeIDentifier string `json:"volume_id"`
	Name             string `json:"name,omitempty"`
	Organization     string `json:"organization"`
}

// ScalewaySnapshot represents a Scaleway Snapshot
type ScalewaySnapshot struct {
	// Identifier is a unique identifier for the snapshot
	Identifier string `json:"id,omitempty"`

	// Name is a user-defined name for the snapshot
	Name string `json:"name,omitempty"`

	// CreationDate is the creation date of the snapshot
	CreationDate string `json:"creation_date,omitempty"`

	// ModificationDate is the date of the last modification of the snapshot
	ModificationDate string `json:"modification_date,omitempty"`

	// Size is the allocated size of the volume
	Size uint64 `json:"size,omitempty"`

	// Organization is the owner of the snapshot
	Organization string `json:"organization"`

	// State is the current state of the snapshot
	State string `json:"state"`

	// VolumeType is the kind of volume behind the snapshot
	VolumeType string `json:"volume_type"`

	// BaseVolume is the volume from which the snapshot inherits
	BaseVolume ScalewayVolume `json:"base_volume,omitempty"`
}

// ScalewayOneSnapshot represents the response of a GET /snapshots/UUID API call
type ScalewayOneSnapshot struct {
	Snapshot ScalewaySnapshot `json:"snapshot,omitempty"`
}

// ScalewaySnapshots represents a group of Scaleway snapshots
type ScalewaySnapshots struct {
	// Snapshots holds scaleway snapshots of the response
	Snapshots []ScalewaySnapshot `json:"snapshots,omitempty"`
}

// PostSnapshot creates a new snapshot
func (s *ScalewayAPI) PostSnapshot(volumeID string, name string) (string, error) {
	definition := ScalewaySnapshotDefinition{
		VolumeIDentifier: volumeID,
		Name:             name,
		Organization:     s.Organization,
	}
	resp, err := s.PostResponse(s.computeAPI, "snapshots", definition)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusCreated}, resp)
	if err != nil {
		return "", err
	}
	var snapshot ScalewayOneSnapshot

	if err = json.Unmarshal(body, &snapshot); err != nil {
		return "", err
	}
	return snapshot.Snapshot.Identifier, nil
}

// DeleteSnapshot deletes a snapshot
func (s *ScalewayAPI) DeleteSnapshot(snapshotID string) error {
	resp, err := s.DeleteResponse(s.computeAPI, fmt.Sprintf("snapshots/%s", snapshotID))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err := s.handleHTTPError([]int{http.StatusNoContent}, resp); err != nil {
		return err
	}
	return nil
}

// GetSnapshots gets the list of snapshots from the ScalewayAPI
func (s *ScalewayAPI) GetSnapshots() (*[]ScalewaySnapshot, error) {
	query := url.Values{}

	resp, err := s.GetResponsePaginate(s.computeAPI, "snapshots", query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var snapshots ScalewaySnapshots

	if err = json.Unmarshal(body, &snapshots); err != nil {
		return nil, err
	}
	return &snapshots.Snapshots, nil
}

// GetSnapshot gets a snapshot from the ScalewayAPI
func (s *ScalewayAPI) GetSnapshot(snapshotID string) (*ScalewaySnapshot, error) {
	resp, err := s.GetResponsePaginate(s.computeAPI, "snapshots/"+snapshotID, url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var oneSnapshot ScalewayOneSnapshot

	if err = json.Unmarshal(body, &oneSnapshot); err != nil {
		return nil, err
	}
	// FIXME region, arch, owner, title
	return &oneSnapshot.Snapshot, nil
}
