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
  Precedence: attr('number'),
  SourceType: attr('string'),
  Action: attr('string'),
  DefaultAddr: attr('string'),
  DefaultPort: attr('number'),
  Meta: attr(),
  Datacenter: attr('string'),
  CreatedAt: attr('date'),
  UpdatedAt: attr('date'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
});
