import Model from 'ember-data/model';
import attr from 'ember-data/attr';

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
});
