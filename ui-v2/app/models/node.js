import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed, get } from '@ember/object';

import sumOfUnhealthy from 'consul-ui/utils/sumOfUnhealthy';
import hasStatus from 'consul-ui/utils/hasStatus';
// import { belongsTo } from 'ember-data/relationships';
export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';
export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  Address: attr('string'),
  Node: attr('string'),
  Meta: attr(), // arbitrary??
  Services: attr(), // hasMany
  Checks: attr(), // hasMany
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
  TaggedAddresses: attr(), // lan, wan
  // Datacenter: belongsTo('dc'),
  Datacenter: attr('string'),
  Segment: attr(),
  Coord: attr(), // hasMany Vec, Error, Adjustment, Height
  hasStatus: function(status) {
    return hasStatus(get(this, 'Checks'), status);
  },
  isHealthy: computed('Checks', function() {
    return sumOfUnhealthy(get(this, 'Checks')) === 0;
  }),
  isUnhealthy: computed('Checks', function() {
    return sumOfUnhealthy(get(this, 'Checks')) > 0;
  }),
});
