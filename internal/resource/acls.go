// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package resource

import "github.com/hashicorp/consul/acl"

// NoOpACLListHook is a common function that can be used if no special list permission is required for a resource.
func NoOpACLListHook(_ acl.Authorizer, _ *acl.AuthorizerContext) error {
	// No-op List permission as we want to default to filtering resources
	// from the list using the Read enforcement.
	return nil
}
