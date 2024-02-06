/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export const ACCESS_LEVEL = {
  GLOBALREADONLY: 'CONSUL_ACCESS_LEVEL_GLOBAL_READ_ONLY',
  GLOBALREADWRITE: 'CONSUL_ACCESS_LEVEL_GLOBAL_READ_WRITE',
};

export default class LinkToHcpModalComponent extends Component {
  @service('repository/token') tokenRepo;
  @service('repository/policy') policyRepo;
  @service('hcp-link-modal') hcpLinkModal;
  @service('router') router;

  @tracked
  token = '';
  @tracked
  accessLevel = ACCESS_LEVEL.GLOBALREADWRITE;
  @tracked
  isGeneratingToken = false;
  AccessLevel = ACCESS_LEVEL;

  get isReadOnlyAccessLevelSelected() {
    return this.accessLevel === this.AccessLevel.GLOBALREADONLY;
  }

  get isTokenGenerated() {
    return this.token && this.token.length > 0;
  }

  deactivateModal = () => {
    this.hcpLinkModal.hide();
  };

  onGenerateTokenClicked = (policy) => {
    this.isGeneratingToken = true;
    let token = this.tokenRepo.create({
      Datacenter: this.args.dc,
      Partition: this.args.partition,
      Namespace: this.args.nspace,
      Policies: [policy.data],
    });
    this.tokenRepo.persist(token, event).then((token) => {
      this.token = token.SecretID;
      this.isGeneratingToken = false;
    });
  };

  @action
  onCancel() {
    this.deactivateModal();
  }
  @action
  onAccessModeChanged({ target }) {
    this.accessLevel = target.value;
  }
}
