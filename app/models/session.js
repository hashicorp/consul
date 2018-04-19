import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export default Model.extend({
  Name: attr('string'),
  ID: attr('string'),
  Node: attr('string'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
  LockDelay: attr('number'),
  Behavior: attr('string'),
  TTL: attr('number'),
  Checks: attr(),
});
