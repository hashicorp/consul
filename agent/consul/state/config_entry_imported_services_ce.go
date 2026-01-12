// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package state

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto/private/pbconfigentry"
)

func prepareImportedServicesResponse(importedServices []importedService, entMeta *acl.EnterpriseMeta) []*pbconfigentry.ImportedService {

	resp := make([]*pbconfigentry.ImportedService, len(importedServices))

	for idx, svc := range importedServices {
		resp[idx] = &pbconfigentry.ImportedService{
			Service:    svc.service,
			SourcePeer: svc.peer,
		}
	}

	return resp
}
