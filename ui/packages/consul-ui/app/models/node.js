import Model, { attr, hasMany } from '@ember-data/model';
import { computed } from '@ember/object';
import { filter } from '@ember/object/computed';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default class Node extends Model {
  @attr('string') uid;
  @attr('string') ID;

  @attr('string') Datacenter;
  @attr('string') Partition;
  @attr('string') Address;
  @attr('string') Node;
  @attr('number') SyncTime;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
  @attr() meta; // {}
  @attr() Meta; // {}
  @attr() TaggedAddresses; // {lan, wan}
  @attr({ defaultValue: () => [] }) Resources; // []
  // Services are reshaped to a different shape to what you sometimes get from
  // the response, see models/node.js
  @hasMany('service-instance') Services; // TODO: Rename to ServiceInstances
  @fragmentArray('health-check') Checks;
  // MeshServiceInstances are all instances that aren't connect-proxies this
  // currently includes gateways as these need to show up in listings
  @filter('Services', item => item.Service.Kind !== 'connect-proxy') MeshServiceInstances;

  @computed('Checks.[]', 'ChecksCritical', 'ChecksPassing', 'ChecksWarning')
  get Status() {
    switch (true) {
      case this.ChecksCritical !== 0:
        return 'critical';
      case this.ChecksWarning !== 0:
        return 'warning';
      case this.ChecksPassing !== 0:
        return 'passing';
      default:
        return 'empty';
    }
  }

  @computed('Checks.[]')
  get ChecksCritical() {
    return this.Checks.filter(item => item.Status === 'critical').length;
  }

  @computed('Checks.[]')
  get ChecksPassing() {
    return this.Checks.filter(item => item.Status === 'passing').length;
  }

  @computed('Checks.[]')
  get ChecksWarning() {
    return this.Checks.filter(item => item.Status === 'warning').length;
  }
}
