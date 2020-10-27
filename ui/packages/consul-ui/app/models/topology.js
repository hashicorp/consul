import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ServiceName';
export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  Datacenter: attr('string'),
  Namespace: attr('string'),
  Upstreams: attr(),
  Downstreams: attr(),
  Protocol: attr(),
  FilteredByACLs: attr(),
  meta: attr(),
});
