import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed } from '@ember/object';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  Address: attr('string'),
  Node: attr('string'),
  Meta: attr(),
  Services: attr(),
  Checks: attr(),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
  TaggedAddresses: attr(),
  Datacenter: attr('string'),
  Segment: attr(),
  Coord: attr(),
  SyncTime: attr('number'),
  meta: attr(),
  Status: computed('Checks.[]', 'ChecksCritical', 'ChecksPassing', 'ChecksWarning', function() {
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
  }),
  ChecksCritical: computed('Checks.[]', function() {
    return this.Checks.filter(item => item.Status === 'critical').length;
  }),
  ChecksPassing: computed('Checks.[]', function() {
    return this.Checks.filter(item => item.Status === 'passing').length;
  }),
  ChecksWarning: computed('Checks.[]', function() {
    return this.Checks.filter(item => item.Status === 'warning').length;
  }),
});
