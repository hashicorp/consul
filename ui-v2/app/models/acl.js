import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  Name: attr('string', {
    // TODO: Why didn't I have to do this for KV's?
    // this is to ensure that Name is '' and not null when creating
    // maybe its due to the fact that `Key` is the primaryKey in Kv's
    defaultValue: '',
  }),
  Type: attr('string'),
  Rules: attr('string'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
  Datacenter: attr('string'),
});
