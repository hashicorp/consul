// +build !consulent

package structs

func (req *ConnectAuthorizeRequest) TargetNamespace() string {
	return IntentionDefaultNamespace
}
