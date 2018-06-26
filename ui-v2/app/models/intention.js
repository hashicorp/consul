import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import writable from 'consul-ui/utils/model/writable';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'ID';
const model = Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  Description: attr('string'),
  SourceNS: attr('string'),
  SourceName: attr('string'),
  DestinationName: attr('string'),
  Precedence: attr('number'),
  SourceType: attr('string', { defaultValue: 'consul' }),
  Action: attr('string', { defaultValue: 'deny' }),
  DefaultAddr: attr('string'),
  DefaultPort: attr('number'),
  Meta: attr(),
  Datacenter: attr('string'),
  CreatedAt: attr('date'),
  UpdatedAt: attr('date'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
});
export const ATTRS = writable(model, [
  'Action',
  'SourceName',
  'DestinationName',
  'SourceType',
  'Description',
]);
export default model;
