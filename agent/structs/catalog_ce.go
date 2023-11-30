// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package structs

func IsConsulServiceID(id ServiceID) bool {
	return id.ID == ConsulServiceID
}

func IsSerfCheckID(id CheckID) bool {
	return id.ID == SerfCheckID
}
