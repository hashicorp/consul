/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

export default class ConsulServiceListItem extends Component {
  get linkParams() {
    const hash = {};

    if (this.args.item.Partition && this.args.partition !== this.args.item.Partition) {
      hash.partition = this.args.item.Partition;
      hash.nspace = this.args.Namespace;
    } else if (this.args.item.Namespace && this.args.nspace !== this.args.item.Namespace) {
      hash.nspace = this.args.item.Namespace;
    }

    if (this.args.item.PeerName) {
      hash.peer = this.args.item.PeerName;
    }

    return hash;
  }
}
