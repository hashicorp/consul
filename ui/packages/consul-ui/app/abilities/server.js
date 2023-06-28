/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import BaseAbility from './base';

export default class ServerAbility extends BaseAbility {
  resource = 'operator';
  segmented = false;
}
