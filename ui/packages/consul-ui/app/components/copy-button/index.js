/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import chart from './chart.xstate';

export default class CopyButton extends Component {
  constructor() {
    super(...arguments);
    this.chart = chart;
  }
}
