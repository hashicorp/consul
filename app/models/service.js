// import { collect, sum, bool, equal } from '@ember/object/computed';
import { computed } from '@ember/object';
import Entity from 'ember-data/model';
import attr from 'ember-data/attr';
import { belongsTo } from 'ember-data/relationships';
// import { fragmentArray } from 'ember-data-model-fragments/attributes';
// import sumAggregation from '../utils/properties/sum-aggregation';
export default Entity.extend({
  Id: attr('string'), // added by ember
  Name: attr('string'),
  ChecksPassing: attr(),
  ChecksCritical: attr(),
  ChecksWarning: attr(),
  Nodes: attr(),
  Datacenter: belongsTo('service'),

  Node: attr(),
  Service: attr(),
  Checks: attr(),
  // Boolean of whether or not there are failing checks in the service.
  // This is used to set color backgrounds and so on.
  hasFailingChecks: computed.gt('failingChecks', 0),
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
  // The number of passing checks within the service.
  passingChecks: computed('ChecksPassing', 'Checks', function() {
    // If the service was returned from `/v1/internal/ui/services`
    // then we have a aggregated value which we can just grab
    if (this.get('ChecksPassing') !== undefined) {
      return this.get('ChecksPassing');
      // Otherwise, we need to filter the child checks by both failing
      // states
    } else {
      return this.get('Checks')
        .filterBy('Status', 'passing')
        .get('length');
    }
  }),
  // The formatted message returned for the user which represents the
  // number of checks failing or passing. Returns `1 passing` or `2 failing`
  checkMessage: computed('passingChecks', 'failingChecks', function() {
    if (this.get('hasFailingChecks') === false) {
      return this.get('passingChecks') + ' passing';
    } else {
      return this.get('failingChecks') + ' failing';
    }
  }),
  nodes: computed.alias('Nodes'),
  // Key used for filtering through an array of this model, i.e s
  // searching
  filterKey: computed.alias('Name'),
});
