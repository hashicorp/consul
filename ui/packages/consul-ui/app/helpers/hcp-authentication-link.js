/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

/**
 * A HCP URL looks like:
 * https://portal.cloud.hashicorp.com/services/consul/clusters/self-managed/link-existing?cluster_name=test-from-api&cluster_version=1.18.0&cluster_access_mode=CONSUL_ACCESS_LEVEL_GLOBAL_READ_WRITE&redirect_url=localhost:8500/services
 */
export const HCP_PREFIX =
  'https://portal.cloud.hashicorp.com/services/consul/clusters/self-managed/link-existing';
export default class hcpAuthenticationLink extends Helper {
  @service('env') env;
  compute([datacenterName, accessMode]) {
    let url = new URL(HCP_PREFIX);
    const clusterVersion = this.env.var('CONSUL_VERSION');

    if (datacenterName) {
      url.searchParams.append('cluster_name', datacenterName);
    }

    if (clusterVersion) {
      url.searchParams.append('cluster_version', clusterVersion);
    }
    if (accessMode) {
      url.searchParams.append('cluster_access_mode', accessMode);
    }

    return url.toString();
  }
}
