/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class LinkToHcpBannerComponent extends Component {
  @service('hcp-link-status') hcpLinkStatus;
  @service('env') env;

  @action
  onDismiss() {
    this.hcpLinkStatus.dismissHcpLinkBanner();
  }
  @action
  onClusterLink() {
    // TODO: CC-7147: Open simplified modal
  }
}
