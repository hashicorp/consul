/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

export default class TopologyMetrics extends Component {
  // =methods
  get hrefPath() {
    const source = this.args.item?.Source;

    return source === 'routing-config' ? 'dc.routing-config' : 'dc.services.show.index';
  }
}
