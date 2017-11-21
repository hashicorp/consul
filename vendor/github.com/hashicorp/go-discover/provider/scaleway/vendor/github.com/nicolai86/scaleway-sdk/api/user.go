package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ScalewayTokenDefinition represents a Scaleway Token
type ScalewayTokenDefinition struct {
	UserID             string                 `json:"user_id"`
	Description        string                 `json:"description,omitempty"`
	Roles              ScalewayRoleDefinition `json:"roles"`
	Expires            string                 `json:"expires"`
	InheritsUsersPerms bool                   `json:"inherits_user_perms"`
	ID                 string                 `json:"id"`
}

// ScalewayTokensDefinition represents a Scaleway Tokens
type ScalewayTokensDefinition struct {
	Token ScalewayTokenDefinition `json:"token"`
}

// ScalewayGetTokens represents a list of Scaleway Tokens
type ScalewayGetTokens struct {
	Tokens []ScalewayTokenDefinition `json:"tokens"`
}

// ScalewayRoleDefinition represents a Scaleway Token UserId Role
type ScalewayRoleDefinition struct {
	Organization ScalewayOrganizationDefinition `json:"organization,omitempty"`
	Role         string                         `json:"role,omitempty"`
}

// ScalewayUserDefinition represents a Scaleway User
type ScalewayUserDefinition struct {
	Email         string                           `json:"email"`
	Firstname     string                           `json:"firstname"`
	Fullname      string                           `json:"fullname"`
	ID            string                           `json:"id"`
	Lastname      string                           `json:"lastname"`
	Organizations []ScalewayOrganizationDefinition `json:"organizations"`
	Roles         []ScalewayRoleDefinition         `json:"roles"`
	SSHPublicKeys []ScalewayKeyDefinition          `json:"ssh_public_keys"`
}

// ScalewayKeyDefinition represents a key
type ScalewayKeyDefinition struct {
	Key         string `json:"key"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// ScalewayUsersDefinition represents the response of a GET /user
type ScalewayUsersDefinition struct {
	User ScalewayUserDefinition `json:"user"`
}

// ScalewayUserPatchSSHKeyDefinition represents a User Patch
type ScalewayUserPatchSSHKeyDefinition struct {
	SSHPublicKeys []ScalewayKeyDefinition `json:"ssh_public_keys"`
}

// PatchUserSSHKey updates a user
func (s *ScalewayAPI) PatchUserSSHKey(UserID string, definition ScalewayUserPatchSSHKeyDefinition) error {
	resp, err := s.PatchResponse(AccountAPI, fmt.Sprintf("users/%s", UserID), definition)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if _, err := s.handleHTTPError([]int{http.StatusOK}, resp); err != nil {
		return err
	}
	return nil
}

// GetUserID returns the userID
func (s *ScalewayAPI) GetUserID() (string, error) {
	resp, err := s.GetResponsePaginate(AccountAPI, fmt.Sprintf("tokens/%s", s.Token), url.Values{})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return "", err
	}
	var token ScalewayTokensDefinition

	if err = json.Unmarshal(body, &token); err != nil {
		return "", err
	}
	return token.Token.UserID, nil
}

// GetUser returns the user
func (s *ScalewayAPI) GetUser() (*ScalewayUserDefinition, error) {
	userID, err := s.GetUserID()
	if err != nil {
		return nil, err
	}
	resp, err := s.GetResponsePaginate(AccountAPI, fmt.Sprintf("users/%s", userID), url.Values{})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := s.handleHTTPError([]int{http.StatusOK}, resp)
	if err != nil {
		return nil, err
	}
	var user ScalewayUsersDefinition

	if err = json.Unmarshal(body, &user); err != nil {
		return nil, err
	}
	return &user.User, nil
}
