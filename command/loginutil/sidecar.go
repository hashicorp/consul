// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

// Package loginutil contains shared constants and helpers used by both the
// login and logout commands to agree on the on-disk contract for the OIDC
// RP-initiated logout sidecar file.
package loginutil

// IDPLogoutSuffix is appended to the token sink file path to derive the path
// of the companion file that stores the OIDC RP-initiated (front-channel)
// logout URL. Both `consul login` (writer) and `consul logout` (reader) must
// use this same suffix to locate the file.
const IDPLogoutSuffix = ".oidc-logout"
