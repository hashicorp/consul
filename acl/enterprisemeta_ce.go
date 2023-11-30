// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package acl

import "hash"

var emptyEnterpriseMeta = EnterpriseMeta{}

// EnterpriseMeta stub
type EnterpriseMeta struct{}

func (m *EnterpriseMeta) ToEnterprisePolicyMeta() *EnterprisePolicyMeta {
	return nil
}

func DefaultEnterpriseMeta() *EnterpriseMeta {
	return &EnterpriseMeta{}
}

func WildcardEnterpriseMeta() *EnterpriseMeta {
	return &EnterpriseMeta{}
}

func (m *EnterpriseMeta) EstimateSize() int {
	return 0
}

func (m *EnterpriseMeta) AddToHash(_ hash.Hash, _ bool) {
	// do nothing
}

func (m *EnterpriseMeta) PartitionOrDefault() string {
	return "default"
}

func EqualPartitions(_, _ string) bool {
	return true
}

func IsDefaultPartition(partition string) bool {
	return true
}

func PartitionOrDefault(_ string) string {
	return "default"
}

func (m *EnterpriseMeta) PartitionOrEmpty() string {
	return ""
}

func (m *EnterpriseMeta) InDefaultPartition() bool {
	return true
}

func (m *EnterpriseMeta) NamespaceOrDefault() string {
	return DefaultNamespaceName
}

func EqualNamespaces(_, _ string) bool {
	return true
}

func NamespaceOrDefault(_ string) string {
	return DefaultNamespaceName
}

func (m *EnterpriseMeta) NamespaceOrEmpty() string {
	return ""
}

func (m *EnterpriseMeta) InDefaultNamespace() bool {
	return true
}

func (m *EnterpriseMeta) Merge(_ *EnterpriseMeta) {
	// do nothing
}

func (m *EnterpriseMeta) MergeNoWildcard(_ *EnterpriseMeta) {
	// do nothing
}

func (_ *EnterpriseMeta) Normalize()          {}
func (_ *EnterpriseMeta) NormalizePartition() {}
func (_ *EnterpriseMeta) NormalizeNamespace() {}

func (m *EnterpriseMeta) Matches(_ *EnterpriseMeta) bool {
	return true
}

func (m *EnterpriseMeta) IsSame(_ *EnterpriseMeta) bool {
	return true
}

func (m *EnterpriseMeta) LessThan(_ *EnterpriseMeta) bool {
	return false
}

func (m *EnterpriseMeta) WithWildcardNamespace() *EnterpriseMeta {
	return &emptyEnterpriseMeta
}

func (m *EnterpriseMeta) UnsetPartition() {
	// do nothing
}

func (m *EnterpriseMeta) OverridePartition(_ string) {
	// do nothing
}

func NewEnterpriseMetaWithPartition(_, _ string) EnterpriseMeta {
	return emptyEnterpriseMeta
}

// FillAuthzContext stub
func (_ *EnterpriseMeta) FillAuthzContext(_ *AuthorizerContext) {}

func NormalizeNamespace(_ string) string {
	return ""
}
