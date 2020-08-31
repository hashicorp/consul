import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed, get } from '@ember/object';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Name';

export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  Tags: attr({
    defaultValue: function() {
      return [];
    },
  }),
  InstanceCount: attr('number'),
  Proxy: attr(),
  ProxyFor: attr(),
  Kind: attr('string'),
  ExternalSources: attr(),
  GatewayConfig: attr(),
  Meta: attr(),
  Address: attr('string'),
  TaggedAddresses: attr(),
  Port: attr('number'),
  EnableTagOverride: attr('boolean'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
  // TODO: These should be typed
  ChecksPassing: attr(),
  ChecksCritical: attr(),
  ChecksWarning: attr(),
  Nodes: attr(),
  Datacenter: attr('string'),
  Namespace: attr('string'),
  Node: attr(),
  Service: attr(),
  Checks: attr(),
  SyncTime: attr('number'),
  meta: attr(),
  /* Mesh properties involve both the service and the associated proxy */
  MeshStatus: computed('MeshChecksPassing', 'MeshChecksWarning', 'MeshChecksCritical', function() {
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
  }),
  MeshChecksPassing: computed('ChecksPassing', 'Proxy.ChecksPassing', function() {
    let proxyCount = 0;
    if (typeof this.Proxy !== 'undefined') {
      proxyCount = this.Proxy.ChecksPassing;
    }
    return this.ChecksPassing + proxyCount;
  }),
  MeshChecksWarning: computed('ChecksWarning', 'Proxy.ChecksWarning', function() {
    let proxyCount = 0;
    if (typeof this.Proxy !== 'undefined') {
      proxyCount = this.Proxy.ChecksWarning;
    }
    return this.ChecksWarning + proxyCount;
  }),
  MeshChecksCritical: computed('ChecksCritical', 'Proxy.ChecksCritical', function() {
    let proxyCount = 0;
    if (typeof this.Proxy !== 'undefined') {
      proxyCount = this.Proxy.ChecksCritical;
    }
    return this.ChecksCritical + proxyCount;
  }),
  /**/
  passing: computed('ChecksPassing', 'Checks', function() {
    let num = 0;
    // TODO: use typeof
    if (get(this, 'ChecksPassing') !== undefined) {
      num = get(this, 'ChecksPassing');
    } else {
      num = get(get(this, 'Checks').filterBy('Status', 'passing'), 'length');
    }
    return {
      length: num,
    };
  }),
  hasStatus: function(status) {
    let num = 0;
    switch (status) {
      case 'passing':
        num = get(this, 'ChecksPassing');
        break;
      case 'critical':
        num = get(this, 'ChecksCritical');
        break;
      case 'warning':
        num = get(this, 'ChecksWarning');
        break;
      case '': // all
        num = 1;
        break;
    }
    return num > 0;
  },
});
