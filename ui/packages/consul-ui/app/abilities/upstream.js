/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import BaseAbility from './base';

export default class UpstreamAbility extends BaseAbility {
  resource = 'upstream';

  get isLinkable() {
    return this.item.InstanceCount > 0;
  }
}
