import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'AccessorID';

export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  IDPName: attr('string'),
  SecretID: attr('string'),
  // Legacy
  Type: attr('string'),
  Name: attr('string', {
    defaultValue: '',
  }),
  Rules: attr('string'),
  // End Legacy
  Legacy: attr('boolean'),
  Description: attr('string', {
    defaultValue: '',
  }),
  Datacenter: attr('string'),
  Local: attr('boolean'),
  Policies: attr({
    defaultValue: function() {
      return [];
    },
  }),
  Roles: attr({
    defaultValue: function() {
      return [];
    },
  }),
  ServiceIdentities: attr({
    defaultValue: function() {
      return [];
    },
  }),
  CreateTime: attr('date'),
  Hash: attr('string'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
});
