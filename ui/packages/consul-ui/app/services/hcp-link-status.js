/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service, { inject as service } from '@ember/service';
import { tracked } from '@glimmer/tracking';

const LOCAL_STORAGE_KEY = 'consul:hideHcpLinkBanner';

export default class HcpLinkStatus extends Service {
  @service abilities;
  @tracked
  userDismissedBanner = false;

  get shouldDisplayBanner() {
    // With the death of Consul Central, we don't want to display the link nav item
    return false;
  }

  get hasPermissionToLink() {
    return this.abilities.can('write operators') && this.abilities.can('write acls');
  }

  constructor() {
    super(...arguments);
    this.userDismissedBanner = !!localStorage.getItem(LOCAL_STORAGE_KEY);
  }

  userHasLinked() {
    // TODO: CC-7145 - once can fetch the link status from the backend, fetch it and set it here
  }

  dismissHcpLinkBanner() {
    localStorage.setItem(LOCAL_STORAGE_KEY, true);
    this.userDismissedBanner = true;
  }
}
