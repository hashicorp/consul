import Entity from 'ember-data/model';
import attr from 'ember-data/attr';
// import { belongsTo } from 'ember-data/relationships';

export default Entity.extend({
  Name: attr('string'),
  ID: attr('string'),
  Node: attr('string'),
  Checks: attr(),
  CreateIndex: attr('number'),
  LockDelay: attr('number'),
});
