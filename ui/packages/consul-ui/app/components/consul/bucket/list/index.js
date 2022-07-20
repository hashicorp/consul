import Component from '@glimmer/component';
import { inject as service } from '@ember/service';

export default class ConsulBucketList extends Component {
  @service abilities;

  get itemsToDisplay() {
    const { item, partition, nspace } = this.args;
    const { abilities } = this;

    let items = [];

    if (partition && abilities.can('use partitions')) {
      if (item.Partition !== partition) {
        if (item.PeerName) {
          this._addPeer(items);
        } else {
          this._addPartition(items);
        }
        this._addNamespace(items);
        this._addService(items);
      } else {
        this._addPeerInfo(items);
      }
    } else if (nspace && abilities.can('use nspaces')) {
      if (item.Namespace !== nspace) {
        this._addPeerInfo(items);
        this._addService(items);
      } else {
        this._addPeerInfo(items);
      }
    } else {
      this._addPeerInfo(items);
    }

    return items;
  }

  _addPeerInfo(items) {
    const { item } = this.args;

    if (item.PeerName) {
      this._addPeer(items);

      if (this.abilities.can('use nspaces')) {
        this._addNamespace(items);
      }
    }
  }

  _addPartition(items) {
    const { item } = this.args;

    items.push({
      type: 'partition',
      label: 'Admin Partition',
      item: item.Partition,
    });
  }

  _addNamespace(items) {
    const { item } = this.args;

    items.push({
      type: 'nspace',
      label: 'Namespace',
      item: item.Namespace,
    });
  }

  _addService(items) {
    const { service, item } = this.args;

    if (service && item.Service) {
      items.push({
        type: 'service',
        label: 'Service',
        item: item.Service,
      });
    }
  }

  _addPeer(items) {
    const { item } = this.args;

    if (item?.PeerName) {
      items.push({
        type: 'peer',
        label: 'Peer',
        item: item.PeerName,
      });
    }
  }
}
