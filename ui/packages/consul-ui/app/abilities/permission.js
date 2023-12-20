/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import BaseAbility from './base';

export default class PermissionAbility extends BaseAbility {
  get canRead() {
    return this.permissions.permissions.length > 0;
  }
}
