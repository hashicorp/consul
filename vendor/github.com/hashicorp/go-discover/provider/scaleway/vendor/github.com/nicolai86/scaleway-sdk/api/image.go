package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ScalewayImageDefinition represents a Scaleway image definition
type ScalewayImageDefinition struct {
	SnapshotIDentifier string  `json:"root_volume"`
	Name               string  `json:"name,omitempty"`
	Organization       string  `json:"organization"`
	Arch               string  `json:"arch"`
	DefaultBootscript  *string `json:"default_bootscript,omitempty"`
}

// ScalewayImage represents a Scaleway Image
type ScalewayImage struct {
	// Identifier is a unique identifier for the image
	Identifier string `json:"id,omitempty"`

	// Name is a user-defined name for the image
	Name string `json:"name,omitempty"`

	// CreationDate is the creation date of the image
	CreationDate string `json:"creation_date,omitempty"`

	// ModificationDate is the date of the last modification of the image
	ModificationDate string `json:"modification_date,omitempty"`

	// RootVolume is the root volume bound to the image
	RootVolume ScalewayVolume `json:"root_volume,omitempty"`

	// Public is true for public images and false for user images
	Public bool `json:"public,omitempty"`

	// Bootscript is the bootscript bound to the image
	DefaultBootscript *ScalewayBootscript `json:"default_bootscript,omitempty"`

	// Organization is the owner of the image
	Organization string `json:"organization,omitempty"`

	// Arch is the architecture target of the image
	Arch string `json:"arch,omitempty"`

	// FIXME: extra_volumes
}

// ScalewayImageIdentifier represents a Scaleway Image Identifier
type ScalewayImageIdentifier struct {
	Identifier string
	Arch       string
	Region     string
	Owner      string
}

// ScalewayOneImage represents the response of a GET /images/UUID API call
type ScalewayOneImage struct {
	Image ScalewayImage `json:"image,omitempty"`
}

// ScalewayImages represents a group of Scaleway images
type ScalewayImages struct {
	// Images holds scaleway images of the response
	Images []ScalewayImage `json:"images,omitempty"`
}

// MarketImages represents MarketPlace images
type MarketImages struct {
	Images []MarketImage `json:"images"`
}

// MarketLocalImageDefinition represents localImage of marketplace version
type MarketLocalImageDefinition struct {
	Arch string `json:"arch"`
	ID   string `json:"id"`
	Zone string `json:"zone"`
}

// MarketLocalImages represents an array of local images
type MarketLocalImages struct {
	LocalImages []MarketLocalImageDefinition `json:"local_images"`
}

// MarketLocalImage represents local image
type MarketLocalImage struct {
	LocalImages MarketLocalImageDefinition `json:"local_image"`
}

// MarketVersionDefinition represents version of marketplace image
type MarketVersionDefinition struct {
	CreationDate string `json:"creation_date"`
	ID           string `json:"id"`
	Image        struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"image"`
	ModificationDate string `json:"modification_date"`
	Name             string `json:"name"`
	MarketLocalImages
}

// MarketVersions represents an array of marketplace image versions
type MarketVersions struct {
	Versions []MarketVersionDefinition `json:"versions"`
}

// MarketVersion represents version of marketplace image
type MarketVersion struct {
	Version MarketVersionDefinition `json:"version"`
}

// MarketImage represents MarketPlace image
type MarketImage struct {
	Categories           []string `json:"categories"`
	CreationDate         string   `json:"creation_date"`
	CurrentPublicVersion string   `json:"current_public_version"`
	Description          string   `json:"description"`
	ID                   string   `json:"id"`
	Logo                 string   `json:"logo"`
	ModificationDate     string   `json:"modification_date"`
	Name                 string   `json:"name"`
	Organization         struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"organization"`
	Public bool `json:"-"`
	MarketVersions
}

// PostImage creates a new image
func (s *ScalewayAPI) PostImage(volumeID string, name string, bootscript string, arch string) (string, error) {
	definition := ScalewayImageDefinition{
		SnapshotIDentifier: volumeID,
		Name:               name,
		Organization:       s.Organization,
		Arch:               arch,
	}
	if bootscript != "" {
		definition.DefaultBootscript = &bootscript
	}

	resp, err := s.PostResponse(s.computeAPI, "images", definition)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusCreated}, resp)
	if err != nil {
		return "", err
	}
	var image ScalewayOneImage

	if err = json.Unmarshal(body, &image); err != nil {
		return "", err
	}
	return image.Image.Identifier, nil
}

// GetImages gets the list of images from the ScalewayAPI
func (s *ScalewayAPI) GetImages() (*[]MarketImage, error) {
	images, err := s.GetMarketPlaceImages("")
	if err != nil {
		return nil, err
	}
	for i, image := range images.Images {
		if image.CurrentPublicVersion != "" {
			for _, version := range image.Versions {
				if version.ID == image.CurrentPublicVersion {
					images.Images[i].Public = true
				}
			}
		}
	}
	values := url.Values{}
	values.Set("organization", s.Organization)
	resp, err := s.GetResponsePaginate(s.computeAPI, "images", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var OrgaImages ScalewayImages

	if err = json.Unmarshal(body, &OrgaImages); err != nil {
		return nil, err
	}

	for _, orgaImage := range OrgaImages.Images {
		images.Images = append(images.Images, MarketImage{
			Categories:           []string{"MyImages"},
			CreationDate:         orgaImage.CreationDate,
			CurrentPublicVersion: orgaImage.Identifier,
			ModificationDate:     orgaImage.ModificationDate,
			Name:                 orgaImage.Name,
			Public:               false,
			MarketVersions: MarketVersions{
				Versions: []MarketVersionDefinition{
					{
						CreationDate:     orgaImage.CreationDate,
						ID:               orgaImage.Identifier,
						ModificationDate: orgaImage.ModificationDate,
						MarketLocalImages: MarketLocalImages{
							LocalImages: []MarketLocalImageDefinition{
								{
									Arch: orgaImage.Arch,
									ID:   orgaImage.Identifier,
									// TODO: fecth images from ams1 and par1
									Zone: s.Region,
								},
							},
						},
					},
				},
			},
		})
	}
	return &images.Images, nil
}

// GetImage gets an image from the ScalewayAPI
func (s *ScalewayAPI) GetImage(imageID string) (*ScalewayImage, error) {
	resp, err := s.GetResponsePaginate(s.computeAPI, "images/"+imageID, url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var oneImage ScalewayOneImage

	if err = json.Unmarshal(body, &oneImage); err != nil {
		return nil, err
	}
	// FIXME owner, title
	return &oneImage.Image, nil
}

// DeleteImage deletes a image
func (s *ScalewayAPI) DeleteImage(imageID string) error {
	resp, err := s.DeleteResponse(s.computeAPI, fmt.Sprintf("images/%s", imageID))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if _, err := s.handleHTTPError([]int{http.StatusNoContent}, resp); err != nil {
		return err
	}
	return nil
}

// GetMarketPlaceImages returns images from marketplace
func (s *ScalewayAPI) GetMarketPlaceImages(uuidImage string) (*MarketImages, error) {
	resp, err := s.GetResponsePaginate(MarketplaceAPI, fmt.Sprintf("images/%s", uuidImage), url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var ret MarketImages

	if uuidImage != "" {
		ret.Images = make([]MarketImage, 1)

		var img MarketImage

		if err = json.Unmarshal(body, &img); err != nil {
			return nil, err
		}
		ret.Images[0] = img
	} else {
		if err = json.Unmarshal(body, &ret); err != nil {
			return nil, err
		}
	}
	return &ret, nil
}

// GetMarketPlaceImageVersions returns image version
func (s *ScalewayAPI) GetMarketPlaceImageVersions(uuidImage, uuidVersion string) (*MarketVersions, error) {
	resp, err := s.GetResponsePaginate(MarketplaceAPI, fmt.Sprintf("images/%v/versions/%s", uuidImage, uuidVersion), url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var ret MarketVersions

	if uuidImage != "" {
		var version MarketVersion
		ret.Versions = make([]MarketVersionDefinition, 1)

		if err = json.Unmarshal(body, &version); err != nil {
			return nil, err
		}
		ret.Versions[0] = version.Version
	} else {
		if err = json.Unmarshal(body, &ret); err != nil {
			return nil, err
		}
	}
	return &ret, nil
}

// GetMarketPlaceImageCurrentVersion return the image current version
func (s *ScalewayAPI) GetMarketPlaceImageCurrentVersion(uuidImage string) (*MarketVersion, error) {
	resp, err := s.GetResponsePaginate(MarketplaceAPI, fmt.Sprintf("images/%v/versions/current", uuidImage), url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var ret MarketVersion

	if err = json.Unmarshal(body, &ret); err != nil {
		return nil, err
	}
	return &ret, nil
}

// GetMarketPlaceLocalImages returns images from local region
func (s *ScalewayAPI) GetMarketPlaceLocalImages(uuidImage, uuidVersion, uuidLocalImage string) (*MarketLocalImages, error) {
	resp, err := s.GetResponsePaginate(MarketplaceAPI, fmt.Sprintf("images/%v/versions/%s/local_images/%s", uuidImage, uuidVersion, uuidLocalImage), url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var ret MarketLocalImages
	if uuidLocalImage != "" {
		var localImage MarketLocalImage
		ret.LocalImages = make([]MarketLocalImageDefinition, 1)

		if err = json.Unmarshal(body, &localImage); err != nil {
			return nil, err
		}
		ret.LocalImages[0] = localImage.LocalImages
	} else {
		if err = json.Unmarshal(body, &ret); err != nil {
			return nil, err
		}
	}
	return &ret, nil
}

// PostMarketPlaceImage adds new image
func (s *ScalewayAPI) PostMarketPlaceImage(images MarketImage) error {
	resp, err := s.PostResponse(MarketplaceAPI, "images/", images)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = s.handleHTTPError([]int{http.StatusAccepted}, resp)
	return err
}

// PostMarketPlaceImageVersion adds new image version
func (s *ScalewayAPI) PostMarketPlaceImageVersion(uuidImage string, version MarketVersion) error {
	resp, err := s.PostResponse(MarketplaceAPI, fmt.Sprintf("images/%v/versions", uuidImage), version)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = s.handleHTTPError([]int{http.StatusAccepted}, resp)
	return err
}

// PostMarketPlaceLocalImage adds new local image
func (s *ScalewayAPI) PostMarketPlaceLocalImage(uuidImage, uuidVersion, uuidLocalImage string, local MarketLocalImage) error {
	resp, err := s.PostResponse(MarketplaceAPI, fmt.Sprintf("images/%v/versions/%s/local_images/%v", uuidImage, uuidVersion, uuidLocalImage), local)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = s.handleHTTPError([]int{http.StatusAccepted}, resp)
	return err
}

// PutMarketPlaceImage updates image
func (s *ScalewayAPI) PutMarketPlaceImage(uudiImage string, images MarketImage) error {
	resp, err := s.PutResponse(MarketplaceAPI, fmt.Sprintf("images/%v", uudiImage), images)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = s.handleHTTPError([]int{http.StatusOK}, resp)
	return err
}

// PutMarketPlaceImageVersion updates image version
func (s *ScalewayAPI) PutMarketPlaceImageVersion(uuidImage, uuidVersion string, version MarketVersion) error {
	resp, err := s.PutResponse(MarketplaceAPI, fmt.Sprintf("images/%v/versions/%v", uuidImage, uuidVersion), version)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = s.handleHTTPError([]int{http.StatusOK}, resp)
	return err
}

// PutMarketPlaceLocalImage updates local image
func (s *ScalewayAPI) PutMarketPlaceLocalImage(uuidImage, uuidVersion, uuidLocalImage string, local MarketLocalImage) error {
	resp, err := s.PostResponse(MarketplaceAPI, fmt.Sprintf("images/%v/versions/%s/local_images/%v", uuidImage, uuidVersion, uuidLocalImage), local)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = s.handleHTTPError([]int{http.StatusOK}, resp)
	return err
}

// DeleteMarketPlaceImage deletes image
func (s *ScalewayAPI) DeleteMarketPlaceImage(uudImage string) error {
	resp, err := s.DeleteResponse(MarketplaceAPI, fmt.Sprintf("images/%v", uudImage))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = s.handleHTTPError([]int{http.StatusNoContent}, resp)
	return err
}

// DeleteMarketPlaceImageVersion delete image version
func (s *ScalewayAPI) DeleteMarketPlaceImageVersion(uuidImage, uuidVersion string) error {
	resp, err := s.DeleteResponse(MarketplaceAPI, fmt.Sprintf("images/%v/versions/%v", uuidImage, uuidVersion))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = s.handleHTTPError([]int{http.StatusNoContent}, resp)
	return err
}

// DeleteMarketPlaceLocalImage deletes local image
func (s *ScalewayAPI) DeleteMarketPlaceLocalImage(uuidImage, uuidVersion, uuidLocalImage string) error {
	resp, err := s.DeleteResponse(MarketplaceAPI, fmt.Sprintf("images/%v/versions/%s/local_images/%v", uuidImage, uuidVersion, uuidLocalImage))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = s.handleHTTPError([]int{http.StatusNoContent}, resp)
	return err
}
