import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  Name: attr('string', {
    defaultValue: '',
  }),
  Description: attr('string', {
    defaultValue: '',
  }),
  Rules: attr('string', {
    defaultValue: '',
  }),
  // frontend only for ordering where CreateIndex can't be used
  CreateTime: attr('date'),
  //
  Datacenter: attr('string'),
  Datacenters: attr(),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),

  template: attr('string', {
    defaultValue: '',
  }),
});
