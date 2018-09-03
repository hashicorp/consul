import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'AccessorID';

export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  SecretID: attr('string'),
  Type: attr('string'), // Legacy only
  Name: attr('string', {
    defaultValue: '',
  }),
  Datacenter: attr('string'),
  Legacy: attr('boolean'),
  Policies: attr(),
  CreateTime: attr('date'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
});
