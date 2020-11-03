import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed } from '@ember/object';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';

export const MANAGEMENT_ID = '00000000-0000-0000-0000-000000000001';

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
  CreateTime: attr('number', {
    defaultValue: function() {
      return new Date().getTime();
    },
  }),
  //
  isGlobalManagement: computed('ID', function() {
    return this.ID === MANAGEMENT_ID;
  }),
  Datacenter: attr('string'),
  Namespace: attr('string'),
  SyncTime: attr('number'),
  meta: attr(),
  Datacenters: attr(),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),

  template: attr('string', {
    defaultValue: '',
  }),
});
