/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default class TemporalFormatHelper extends Helper {
  @service('temporal') temporal;
  compute([value], hash) {
    return this.temporal.format(value, hash);
  }
}
