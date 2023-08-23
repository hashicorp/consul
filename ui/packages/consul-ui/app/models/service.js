import Model, { attr } from '@ember-data/model';
import { computed } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { fragment } from 'ember-data-model-fragments/attributes';
import replace, { nullValue } from 'consul-ui/decorators/replace';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Name';

export const Collection = class Collection {
  @tracked items;

  constructor(items) {
    this.items = items;
  }

  get ExternalSources() {
    const items = this.items.reduce(function(prev, item) {
      return prev.concat(item.ExternalSources || []);
    }, []);
    // unique, non-empty values, alpha sort
    return [...new Set(items)].filter(Boolean).sort();
  }
  // TODO: Think about when this/collections is worthwhile using and explain
  // when and when not somewhere in the docs
  get Partitions() {
    // unique, non-empty values, alpha sort
    return [...new Set(this.items.map(item => item.Partition))].sort();
  }
};
export default class Service extends Model {
  @attr('string') uid;
  @attr('string') Name;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('string') Kind;
  @replace('', undefined) @attr('string') PeerName;
  @attr('number') ChecksPassing;
  @attr('number') ChecksCritical;
  @attr('number') ChecksWarning;
  @attr('number') InstanceCount;
  @attr('boolean') ConnectedWithGateway;
  @attr('boolean') ConnectedWithProxy;
  @attr({ defaultValue: () => [] }) Resources; // []
  @attr('number') SyncTime;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;

  @nullValue([]) @attr({ defaultValue: () => [] }) Tags;

  @attr() Nodes; // array
  @attr() Proxy; // Service
  @fragment('gateway-config') GatewayConfig;
  @nullValue([]) @attr() ExternalSources; // array
  @attr() Meta; // {}

  @attr() meta; // {}

  @computed('ChecksPassing', 'ChecksWarning', 'ChecksCritical')
  get ChecksTotal() {
    return this.ChecksPassing + this.ChecksWarning + this.ChecksCritical;
  }

  @computed('MeshChecksPassing', 'MeshChecksWarning', 'MeshChecksCritical')
  get MeshChecksTotal() {
    return this.MeshChecksPassing + this.MeshChecksWarning + this.MeshChecksCritical;
  }

  /* Mesh properties involve both the service and the associated proxy */
  @computed('ConnectedWithProxy', 'ConnectedWithGateway')
  get MeshEnabled() {
    return this.ConnectedWithProxy || this.ConnectedWithGateway;
  }

  @computed('MeshEnabled', 'Kind')
  get InMesh() {
    return this.MeshEnabled || (this.Kind || '').length > 0;
  }

  @computed('MeshChecksPassing', 'MeshChecksWarning', 'MeshChecksCritical')
  get MeshStatus() {
    switch (true) {
      case this.MeshChecksCritical !== 0:
        return 'critical';
      case this.MeshChecksWarning !== 0:
        return 'warning';
      case this.MeshChecksPassing !== 0:
        return 'passing';
      default:
        return 'empty';
    }
  }

  @computed('ChecksPassing', 'Proxy.ChecksPassing')
  get MeshChecksPassing() {
    let proxyCount = 0;
    if (typeof this.Proxy !== 'undefined') {
      proxyCount = this.Proxy.ChecksPassing;
    }
    return this.ChecksPassing + proxyCount;
  }

  @computed('ChecksWarning', 'Proxy.ChecksWarning')
  get MeshChecksWarning() {
    let proxyCount = 0;
    if (typeof this.Proxy !== 'undefined') {
      proxyCount = this.Proxy.ChecksWarning;
    }
    return this.ChecksWarning + proxyCount;
  }

  @computed('ChecksCritical', 'Proxy.ChecksCritical')
  get MeshChecksCritical() {
    let proxyCount = 0;
    if (typeof this.Proxy !== 'undefined') {
      proxyCount = this.Proxy.ChecksCritical;
    }
    return this.ChecksCritical + proxyCount;
  }
  /**/
}
