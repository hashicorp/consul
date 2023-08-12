/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default class TweenToHelper extends Helper {
  @service('ticker') ticker;

  compute([props, id], hash) {
    return this.ticker.tweenTo(props, id);
  }
}
