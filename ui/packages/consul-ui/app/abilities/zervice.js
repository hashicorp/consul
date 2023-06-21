/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import BaseAbility from './base';

export default class ZerviceAbility extends BaseAbility {
  resource = 'service';

  get isLinkable() {
    return this.item.InstanceCount > 0;
  }

  get canReadIntention() {
    if (typeof this.item === 'undefined' || typeof this.item.Resources === 'undefined') {
      return false;
    }
    const found = this.item.Resources.find(
      (item) => item.Resource === 'intention' && item.Access === 'read' && item.Allow === true
    );
    return typeof found !== 'undefined';
  }

  get canWriteIntention() {
    if (typeof this.item === 'undefined' || typeof this.item.Resources === 'undefined') {
      return false;
    }
    const found = this.item.Resources.find(
      (item) => item.Resource === 'intention' && item.Access === 'write' && item.Allow === true
    );
    return typeof found !== 'undefined';
  }

  get canCreateIntention() {
    return this.canWriteIntention;
  }

  get canUpdateIntention() {
    return this.canWriteIntention;
  }
}
