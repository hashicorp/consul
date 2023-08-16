/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

export default class StateChartHelper extends Helper {
  @service('state') state;

  compute([value], hash) {
    return this.state.stateChart(value);
  }
}
