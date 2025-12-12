// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !windows

package checks

import "fmt"

type OSServiceClient struct {
}

func NewOSServiceClient() (*OSServiceClient, error) {
	return nil, fmt.Errorf("not implemented")
}

func (client *OSServiceClient) Check(serviceName string) error {
	return fmt.Errorf("not implemented")
}
