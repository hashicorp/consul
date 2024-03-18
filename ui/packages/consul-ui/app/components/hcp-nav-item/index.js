/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

/**
 * If the user has accessed consul from HCP managed consul, we do NOT want to display the
 * "HCP Consul Central↗️" link in the nav bar. As we're already displaying a BackLink to HCP.
 */
export default class HcpLinkItemComponent extends Component {
  @service env;

  get shouldShowBackToHcpItem() {
    const isConsulHcpUrlDefined = !!this.env.var('CONSUL_HCP_URL');
    const isConsulHcpEnabled = !!this.env.var('CONSUL_HCP_ENABLED');
    return isConsulHcpEnabled && isConsulHcpUrlDefined;
  }
}
