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
  Kind: attr('string'),
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
  Node: attr(),
  Service: attr(),
  Checks: attr(),
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
