/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default class TemporalWithinHelper extends Helper {
  @service('temporal') temporal;
  compute(params, hash) {
    return this.temporal.within(params, hash);
  }
}
