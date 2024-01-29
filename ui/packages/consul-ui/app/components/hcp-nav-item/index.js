/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

/**
 * If the user has accessed consul from HCP managed consul, we do NOT want to display the
 * "HCP Consul Central↗️" link in the nav bar. As we're already displaying a BackLink to HCP.
 */
export default class HcpLinkItemComponent extends Component {
  @service env;
  @service('hcp-link-status') hcpLinkStatus;

  get alreadyLinked() {
    return this.args.linkData?.isLinked;
  }

  get shouldDisplayNavLinkItem() {
    const alreadyLinked = this.alreadyLinked;
    const undefinedResourceId = !this.args.linkData?.resourceId;
    const unauthorizedToLink = !this.hcpLinkStatus.hasPermissionToLink;
    const undefinedLinkStatus = this.args.linkData?.isLinked === undefined;

    // We need permission to link to display the link nav item
    if (unauthorizedToLink) {
      return false;
    }

    // If the link status is undefined, we don't want to display the link nav item
    if (undefinedLinkStatus) {
      return false;
    }

    // If the user has already linked, but we don't have the resourceId to link them to HCP, we don't want to display the link nav item
    if (alreadyLinked && undefinedResourceId) {
      return false;
    }

    return true;
  }

  get shouldShowBackToHcpItem() {
    const isConsulHcpUrlDefined = !!this.env.var('CONSUL_HCP_URL');
    const isConsulHcpEnabled = !!this.env.var('CONSUL_HCP_ENABLED');
    return isConsulHcpEnabled && isConsulHcpUrlDefined;
  }

  get shouldDisplayNewBadge() {
    return this.hcpLinkStatus.shouldDisplayHcpLinkPrompt;
  }

  @action
  onLinkToConsulCentral() {
    // TODO: https://hashicorp.atlassian.net/browse/CC-7147 open the modal
  }
}
