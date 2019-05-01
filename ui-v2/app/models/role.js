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
  Policies: attr({
    defaultValue: function() {
      return [];
    },
  }),
  ServiceIdentities: attr({
    defaultValue: function() {
      return [];
    },
  }),
  // frontend only for ordering where CreateIndex can't be used
  CreateTime: attr('date'),
  //
  Datacenter: attr('string'),
  // TODO: Figure out whether we need this or not
  Datacenters: attr(),
  Hash: attr('string'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
});
