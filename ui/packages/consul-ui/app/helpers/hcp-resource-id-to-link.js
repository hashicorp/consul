/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';

/**
 * A resourceId Looks like:
 * organization/b4432207-bb9c-438e-a160-b98923efa979/project/4b09958c-fa91-43ab-8029-eb28d8cee9d4/hashicorp.consul.global-network-manager.cluster/test-from-api
 * organization/${organizationId}/project/${projectId}/hashicorp.consul.global-network-manager.cluster/${clusterName}
 *
 * A HCP URL looks like:
 * https://portal.hcp.dev/services/consul/clusters/self-managed/test-from-api?project_id=4b09958c-fa91-43ab-8029-eb28d8cee9d4
 * ${HCP_PREFIX}/${clusterName}?project_id=${projectId}
 */
export const HCP_PREFIX =
  'https://portal.cloud.hashicorp.com/services/consul/clusters/self-managed';
export default class hcpResourceIdToLink extends Helper {
  // TODO: How can we figure out different HCP environments?
  compute([resourceId], hash) {
    let url = HCP_PREFIX;
    // Array looks like: ["organization", organizationId, "project", projectId, "hashicorp.consul.global-network-manager.cluster", "Cluster Id"]
    const [, , , projectId, , clusterName] = resourceId.split('/');
    if (!projectId || !clusterName) {
      return '';
    }

    url += `/${clusterName}?project_id=${projectId}`;
    return url;
  }
}
