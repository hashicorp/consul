import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import writable from 'consul-ui/utils/model/writable';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'AccessorID';

const model = Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  SecretID: attr('string'),
  // Legacy
  Type: attr('string'),
  Name: attr('string', {
    defaultValue: '',
  }),
  // End Legacy
  Description: attr('string', {
    defaultValue: '',
  }),
  Datacenter: attr('string'),
  Legacy: attr('boolean'),
  Local: attr('boolean'),
  Policies: attr({
    defaultValue: function() {
      return [];
    },
  }),
  CreateTime: attr('date'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
});
// Name is only for Legacy tokens, not sure if they get upgraded yet?
export const ATTRS = writable(model, ['Name', 'Description', 'Policies', 'AccessorID']);
export default model;
