/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

export default class TooltipComponent extends Component {
  get tooltipOptions() {
    return {
      triggerTarget: 'parentNode',
      ...this.args.options,
    };
  }
}
