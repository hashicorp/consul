/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';
import { tracked } from '@glimmer/tracking';
import { inject as service } from '@ember/service';

const LOCAL_STORAGE_KEY = 'consul:hideHcpLinkBanner';

export default class HcpLinkStatus extends Service {
  @service('repository/hcpLink') hcpLinkRepo;
  @tracked
  alreadyLinked = false;
  @tracked
  userDismissedBanner = false;
  @tracked
  isAlreadyFetchedLinkingStatus = false;

  get shouldDisplayBanner() {
    return this.isAlreadyFetchedLinkingStatus && !this.alreadyLinked && !this.userDismissedBanner;
  }

  constructor() {
    super(...arguments);
    this.userDismissedBanner = !!localStorage.getItem(LOCAL_STORAGE_KEY);
    this.hcpLinkRepo.fetch().then((hcpLinkedData) => {
      debugger;
      this.isAlreadyFetchedLinkingStatus = true;
      this.alreadyLinked = hcpLinkedData.data?.isLinked;
    });
  }

  userHasLinked() {
    // TODO: CC-7145 - once can fetch the link status from the backend, fetch it and set it here
  }

  dismissHcpLinkBanner() {
    localStorage.setItem(LOCAL_STORAGE_KEY, true);
    this.userDismissedBanner = true;
  }
}
