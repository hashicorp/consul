/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

export default class ConsulServiceList extends Component {
  linkParams = (item) => {
    const hash = {};

    if (item.Partition && this.args.partition !== item.Partition) {
      hash.partition = item.Partition;
      hash.nspace = this.args.Namespace;
    } else if (item.Namespace && this.args.nspace !== item.Namespace) {
      hash.nspace = item.Namespace;
    }

    if (item.PeerName) {
      hash.peer = item.PeerName;
    }

    return hash;
  };
}
