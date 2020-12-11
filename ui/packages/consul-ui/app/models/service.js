import Model, { attr } from '@ember-data/model';
import { computed, get } from '@ember/object';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Name';

export default class Service extends Model {
  @attr('string') uid;
  @attr('string') Name;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Kind;
  @attr('number') ChecksPassing;
  @attr('number') ChecksCritical;
  @attr('number') ChecksWarning;
  @attr('number') InstanceCount;
  @attr('boolean') ConnectedWithGateway;
  @attr('boolean') ConnectedWithProxy;
  @attr('number') SyncTime;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
  @attr({ defaultValue: () => [] }) Tags;

  @attr() Nodes; // array
  @attr() Proxy; // Service
  @attr() GatewayConfig; // {AssociatedServiceCount: 0}
  @attr() ExternalSources; // array
  @attr() Meta; // {}

  @attr() meta; // {}

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
