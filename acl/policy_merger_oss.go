// +build !consulent

package acl

type enterprisePolicyRulesMergeContext struct{}

func (ctx *enterprisePolicyRulesMergeContext) init() {
	// do nothing
}

func (ctx *enterprisePolicyRulesMergeContext) merge(*EnterprisePolicyRules) {
	// do nothing
}

func (ctx *enterprisePolicyRulesMergeContext) update(*EnterprisePolicyRules) {
	// do nothing
}
