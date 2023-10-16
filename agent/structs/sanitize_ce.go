//go:build !consulent
// +build !consulent

package structs

// SanitizeLegacyACLToken does nothing in the OSS builds. It does not mutate
// the input argument at all.
//
// In enterprise builds this hook is necessary to support fixing old multiline
// HCL strings in legacy token Sentinel policies into heredocs. If the token
// was updated and previously had a Hash set, this will also update it.
//
// DEPRECATED (ACL-Legacy-Compat)
func SanitizeLegacyACLToken(token *ACLToken) {
}

// SanitizeLegacyACLTokenRules does nothing in the OSS builds. It always
// returns an empty string.
//
// In enterprise builds this hook is necessary to support fixing any old
// multiline HCL strings in legacy token Sentinel policies into heredocs.
//
// DEPRECATED (ACL-Legacy-Compat)
func SanitizeLegacyACLTokenRules(rules string) string {
	return ""
}
