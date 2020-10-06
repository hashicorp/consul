import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed } from '@ember/object';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ServiceName';
export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  Datacenter: attr('string'),
  Namespace: attr('string'),
  Upstreams: attr(),
  Downstreams: attr(),
  meta: attr(),
  Exists: computed(function() {
    return true;
  }),
});
