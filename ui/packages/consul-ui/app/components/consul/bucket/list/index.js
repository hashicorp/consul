/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class ConsulBucketList extends Component {
  @service abilities;

  get itemsToDisplay() {
    const { peerOrPartitionPart, namespacePart, servicePart } = this;

    return [...peerOrPartitionPart, ...namespacePart, ...servicePart];
  }

  get peerOrPartitionPart() {
    const { peerPart, partitionPart } = this;

    if (peerPart.length) {
      return peerPart;
    } else {
      return partitionPart;
    }
  }

  get partitionPart() {
    const { item, partition } = this.args;

    const { abilities } = this;

    if (partition && abilities.can('use partitions')) {
      if (item.Partition !== partition) {
        return [
          {
            type: 'partition',
            label: 'Admin Partition',
            item: item.Partition,
          },
        ];
      }
    }

    return [];
  }

  get peerPart() {
    const { item } = this.args;

    if (item.PeerName) {
      return [
        {
          type: 'peer',
          label: 'Peer',
          item: item.PeerName,
        },
      ];
    }

    return [];
  }

  get namespacePart() {
    const { item, nspace } = this.args;
    const { abilities, partitionPart, peerPart } = this;

    const nspaceItem = {
      type: 'nspace',
      label: 'Namespace',
      item: item.Namespace,
    };

    // when we surface a partition - show a namespace with it
    if (partitionPart.length) {
      return [nspaceItem];
    }

    if (peerPart.length && abilities.can('use nspaces')) {
      return [nspaceItem];
    }

    if (nspace && abilities.can('use nspaces')) {
      if (item.Namespace !== nspace) {
        return [nspaceItem];
      }
    }

    return [];
  }

  get servicePart() {
    const { item, service } = this.args;

    const { partitionPart, namespacePart } = this;

    // when we show partitionPart or namespacePart -> consider service part
    if (partitionPart.length || namespacePart.length) {
      if (item.Service && service) {
        return [
          {
            type: 'service',
            label: 'Service',
            item: item.Service,
          },
        ];
      }
    }

    return [];
  }
}
