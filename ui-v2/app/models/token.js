import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed } from '@ember/object';
import { MANAGEMENT_ID } from 'consul-ui/models/policy';

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
  meta: attr(),
  Datacenter: attr('string'),
  Namespace: attr('string'),
  Local: attr('boolean'),
  isGlobalManagement: computed('Policies.[]', function() {
    return (this.Policies || []).find(item => item.ID === MANAGEMENT_ID);
  }),
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
  NodeIdentities: attr({
    defaultValue: function() {
      return [];
    },
  }),
  CreateTime: attr('date'),
  Hash: attr('string'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
});
