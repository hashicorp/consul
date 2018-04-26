import Model from 'ember-data/model';
import attr from 'ember-data/attr';
// import { belongsTo } from 'ember-data/relationships';
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
  Address: attr('string'),
  Port: attr('number'),
  EnableTagOverride: attr('boolean'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
  ChecksPassing: attr(),
  ChecksCritical: attr(),
  ChecksWarning: attr(),
  Nodes: attr(),
  Datacenter: attr('string'),
  // Datacenter: belongsTo('dc'),

  Node: attr(),
  Service: attr(),
  Checks: attr(),
  // The number of failing checks within the service.
  failingChecks: computed('ChecksCritical', 'ChecksWarning', 'Checks', function() {
    // If the service was returned from `/v1/internal/ui/services`
    // then we have a aggregated value which we can just grab
    if (get(this, 'ChecksCritical') !== undefined) {
      return get(this, 'ChecksCritical') + get(this, 'ChecksWarning');
      // Otherwise, we need to filter the child checks by both failing
      // states
    } else {
      var checks = get(this, 'Checks');
      return (
        get(checks.filterBy('Status', 'critical'), 'length') +
        get(checks.filterBy('Status', 'warning'), 'length')
      );
    }
  }),
  passing: computed('ChecksPassing', 'Checks', function() {
    let num = 0;
    if (get(this, 'ChecksPassing') !== undefined) {
      // TODO: if we don't need this then just return the filterBy array
      // as it has a length
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
