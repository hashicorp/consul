// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

import "fmt"

// validateCredentialInjection rejects terminating-gateway credential injection
// on Consul CE: it is a Consul Enterprise feature. CE carries the types so API
// consumers (such as consul-k8s) compile against the same surface, but it
// neither stores nor renders credential injection configuration.
func (e *TerminatingGatewayConfigEntry) validateCredentialInjection() error {
	if e.CredentialInjection != nil {
		return fmt.Errorf("credential injection is a consul enterprise feature")
	}
	for _, svc := range e.Services {
		if svc.Credential != nil {
			return fmt.Errorf("service %q: credential injection is a consul enterprise feature", svc.Name)
		}
	}
	return nil
}
