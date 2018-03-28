// import { collect, sum, bool, equal } from '@ember/object/computed';
import Model from 'ember-data/model';
import attr from 'ember-data/attr';
// import { belongsTo } from 'ember-data/relationships';
import { computed } from '@ember/object';
// import { fragmentArray } from 'ember-data-model-fragments/attributes';
// import sumAggregation from '../utils/properties/sum-aggregation';
export default Model.extend({
  Id: attr('string'), // added by ember
  Name: attr('string'),
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

  Tags: attr(),
  Node: attr(),
  Service: attr(),
  Checks: attr(),
  // The number of failing checks within the service.
  failingChecks: computed('ChecksCritical', 'ChecksWarning', 'Checks', function() {
    // If the service was returned from `/v1/internal/ui/services`
    // then we have a aggregated value which we can just grab
    if (this.get('ChecksCritical') !== undefined) {
      return this.get('ChecksCritical') + this.get('ChecksWarning');
      // Otherwise, we need to filter the child checks by both failing
      // states
    } else {
      var checks = this.get('Checks');
      return (
        checks.filterBy('Status', 'critical').get('length') +
        checks.filterBy('Status', 'warning').get('length')
      );
    }
  }),
  passing: computed('ChecksPassing', 'Checks', function() {
    let num = 0;
    if (this.get('ChecksPassing') !== undefined) {
      // TODO: if we don't need this then just return the filterBy array
      // as it has a length
      num = this.get('ChecksPassing');
    } else {
      num = this.get('Checks')
        .filterBy('Status', 'passing')
        .get('length');
    }
    return {
      length: num,
    };
  }),
  hasStatus: function(status) {
    let num = 0;
    switch (status) {
      case 'passing':
        num = this.get('ChecksPassing');
        break;
      case 'critical':
        num = this.get('ChecksCritical');
        break;
      case 'warning':
        num = this.get('ChecksWarning');
        break;
      case '': // all
        num = 1;
        break;
    }
    return num > 0;
  },
});
