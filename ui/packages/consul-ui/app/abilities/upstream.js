/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import BaseAbility from './base';

export default class UpstreamAbility extends BaseAbility {
  resource = 'upstream';

  get isLinkable() {
    return this.item.InstanceCount > 0;
  }
}
