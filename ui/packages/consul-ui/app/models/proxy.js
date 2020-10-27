import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Node,ServiceID';
export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  ID: attr('string'),
  ServiceName: attr('string'),
  ServiceID: attr('string'),
  Node: attr('string'),
  ServiceProxy: attr(),
  SyncTime: attr('number'),
  Datacenter: attr('string'),
  Namespace: attr('string'),
});
