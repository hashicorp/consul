import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  Name: attr('string'),
  Node: attr('string'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
  LockDelay: attr('number'),
  Behavior: attr('string'),
  TTL: attr('string'),
  Checks: attr({
    defaultValue: function() {
      return [];
    },
  }),
  Datacenter: attr('string'),
});
