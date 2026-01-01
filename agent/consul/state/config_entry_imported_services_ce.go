// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package state

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto/private/pbconfigentry"
)

func prepareImportedServicesResponse(importedServices []importedService, entMeta *acl.EnterpriseMeta) []*pbconfigentry.ResolvedImportedService {

	resp := make([]*pbconfigentry.ResolvedImportedService, len(importedServices))

	for idx, importedService := range importedServices {
		resp[idx] = &pbconfigentry.ResolvedImportedService{
			Service:    importedService.service,
			SourcePeer: importedService.peer,
		}
	}

	return resp
}
