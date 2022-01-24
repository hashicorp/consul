//go:build !consulent
// +build !consulent

package structs

func validateSourceIntentionEnterpriseMeta(_, _ *EnterpriseMeta) error {
	return nil
}
