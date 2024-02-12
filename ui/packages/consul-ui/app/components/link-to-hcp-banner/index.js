/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class LinkToHcpBannerComponent extends Component {
  @service('hcp-link-status') hcpLinkStatus;
  @service('hcp-link-modal') hcpLinkModal;
  @service('env') env;

  get notLinked() {
    return this.args.linkData?.isLinked === false;
  }

  @action
  onDismiss() {
    this.hcpLinkStatus.dismissHcpLinkBanner();
  }
  @action
  onClusterLink() {
    this.hcpLinkModal.setResourceId(this.args.linkData?.resourceId);
    this.hcpLinkModal.show();
  }
}
