import Model, { attr, hasMany } from '@ember-data/model';
import { computed } from '@ember/object';
import { fragmentArray } from 'ember-data-model-fragments/attributes';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default class Node extends Model {
  @attr('string') uid;
  @attr('string') ID;

  @attr('string') Datacenter;
  @attr('string') Address;
  @attr('string') Node;
  @attr('number') SyncTime;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
  @attr() meta; // {}
  @attr() Meta; // {}
  @attr() TaggedAddresses; // {lan, wan}
  @attr({ defaultValue: () => [] }) Resources; // []
  @hasMany('service-instance') Services; // TODO: Rename to ServiceInstances
  @fragmentArray('health-check') Checks;

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
