import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed } from '@ember/object';

import { fragmentArray } from 'ember-data-model-fragments/attributes';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';
export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  Description: attr('string'),
  SourceNS: attr('string'),
  SourceName: attr('string', { defaultValue: '*' }),
  DestinationName: attr('string', { defaultValue: '*' }),
  DestinationNS: attr('string'),
  Precedence: attr('number'),
  Permissions: fragmentArray('intention-permission'),
  SourceType: attr('string', { defaultValue: 'consul' }),
  Action: attr('string'),
  Meta: attr(),
  Legacy: attr('boolean', { defaultValue: true }),
  LegacyID: attr('string'),

  IsManagedByCRD: computed('Meta', function() {
    const meta = Object.entries(this.Meta || {}).find(
      ([key, value]) => key === 'external-source' && value === 'kubernetes'
    );
    return typeof meta !== 'undefined';
  }),
  IsEditable: computed('Legacy', 'IsManagedByCRD', function() {
    return this.Legacy && !this.IsManagedByCRD;
  }),
  SyncTime: attr('number'),
  Datacenter: attr('string'),
  CreatedAt: attr('date'),
  UpdatedAt: attr('date'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
});
