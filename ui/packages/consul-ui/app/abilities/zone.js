/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import BaseAbility from './base';
import { inject as service } from '@ember/service';

export default class ZoneAbility extends BaseAbility {
  @service('env') env;

  get canRead() {
    return this.env.var('CONSUL_NSPACES_ENABLED');
  }
}
