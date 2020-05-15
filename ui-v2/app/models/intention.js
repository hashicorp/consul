import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';
export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  Description: attr('string'),
  SourceNS: attr('string'),
  SourceName: attr('string'),
  DestinationName: attr('string'),
  DestinationNS: attr('string'),
  Precedence: attr('number'),
  SourceType: attr('string', { defaultValue: 'consul' }),
  Action: attr('string', { defaultValue: 'deny' }),
  Meta: attr(),
  SyncTime: attr('number'),
  Datacenter: attr('string'),
  CreatedAt: attr('date'),
  UpdatedAt: attr('date'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
});
