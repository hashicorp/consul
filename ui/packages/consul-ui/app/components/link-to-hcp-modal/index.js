/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

export default class LinkToHcpModalComponent extends Component {
  @service('repository/token') tokenRepo;
  @service('repository/policy') policyRepo;
  @tracked
  token = '';
  @tracked
  accessLevel;
  @tracked
  isGeneratingToken = false;

  get isReadOnlyAccessLevelSelected() {
    return this.accessLevel === 'READONLY';
  }

  get isTokenGenerated() {
    return this.token && this.token.length > 0;
  }

  deactivateModal() {
    // TODO: call input function onCancel
  }

  @action
  onCancel() {
    // TODO: add on cancel modal
  }
  @action
  onGenerateTokenClicked(event) {
    this.isGeneratingToken = true;
    // TODO: check why policy is not set
    let token = this.tokenRepo.create({
      Datacenter: this.args.dc,
      Partition: this.args.partition,
      Namespace: this.args.nspace,
      Policies: [this.args.policy.data],
    });
    this.tokenRepo.persist(token, event).then((token) => {
      this.token = token.SecretID;
      this.isGeneratingToken = false;
    });
  }
  @action
  onAccessModeChanged({ target }) {
    this.accessLevel = target.value;
  }
}
