/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import BaseAbility from './base';

export default class ServerAbility extends BaseAbility {
  resource = 'operator';
  segmented = false;
}
