/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import BaseAbility from 'consul-ui/abilities/base';
import { inject as service } from '@ember/service';

export default class PeerAbility extends BaseAbility {
  @service('env') env;

  resource = 'peering';
  segmented = false;

  get isLinkable() {
    return this.canDelete;
  }
  get canDelete() {
    return !['DELETING'].includes(this.item.State) && super.canDelete;
  }

  get canUse() {
    return this.env.var('CONSUL_PEERINGS_ENABLED');
  }
}
