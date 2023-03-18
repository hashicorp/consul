/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import BaseAbility from 'consul-ui/abilities/base';
import { inject as service } from '@ember/service';

export default class PartitionAbility extends BaseAbility {
  @service('env') env;
  @service('repository/dc') dcs;

  resource = 'operator';
  segmented = false;

  get isLinkable() {
    return !this.item.DeletedAt;
  }

  get canManage() {
    // management currently means "can I write", not necessarily just create
    return this.canWrite;
  }

  get canCreate() {
    // we can only currently create a partition if you have only one datacenter
    if (this.dcs.peekAll().length > 1) {
      return false;
    }
    return super.canCreate;
  }

  get canDelete() {
    return this.item.Name !== 'default' && super.canDelete;
  }

  get canChoose() {
    if (typeof this.dc === 'undefined') {
      return false;
    }
    return this.canUse && this.dc.Primary;
  }

  get canUse() {
    return this.env.var('CONSUL_PARTITIONS_ENABLED');
  }
}
