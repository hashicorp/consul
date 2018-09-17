import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import writable from 'consul-ui/utils/model/writable';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'AccessorID';

const model = Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  SecretID: attr('string'),
  Type: attr('string'), // Legacy only
  Name: attr('string', {
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
export const ATTRS = writable(model, ['Name', 'Policies']);
export default model;
