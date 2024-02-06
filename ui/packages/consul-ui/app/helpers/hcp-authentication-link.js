/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

/**
 * A resourceId Looks like:
 * organization/b4432207-bb9c-438e-a160-b98923efa979/project/4b09958c-fa91-43ab-8029-eb28d8cee9d4/hashicorp.consul.global-network-manager.cluster/test-from-api
 * organization/${organizationId}/project/${projectId}/hashicorp.consul.global-network-manager.cluster/${clusterName}
 *
 * A HCP URL looks like:
 * https://portal.cloud.hashicorp.com/sign-in?cluster_name=test-from-api&redirect_to=link-consul-cluster&cluster_version=1.18.0&cluster_access_mode=CONSUL_ACCESS_LEVEL_GLOBAL_READ_WRITE&redirect_url=localhost:8500/services
 */
export const HCP_PREFIX =
  'https://portal.cloud.hashicorp.com/sign-in?redirect_to=link-consul-cluster';
export default class hcpAuthenticationLink extends Helper {
  @service('env') env;
  compute([resourceId, accessMode], hash) {
    if (!resourceId) {
      return;
    }

    let url = new URL(HCP_PREFIX);
    const clusterVersion = this.env.var('CONSUL_VERSION');

    // Array looks like: ["organization", organizationId, "project", projectId, "hashicorp.consul.global-network-manager.cluster", "Cluster Id"]
    const [, , , projectId, , clusterName] = resourceId.split('/');
    if (!projectId || !clusterName) {
      return '';
    }

    url.searchParams.append('cluster_name', clusterName);
    if (clusterVersion) {
      url.searchParams.append('cluster_version', clusterVersion);
    }
    if (accessMode) {
      url.searchParams.append('cluster_access_mode', accessMode);
    }

    return url.toString();
  }
}
