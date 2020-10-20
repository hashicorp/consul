import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Name';
export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  meta: attr(),
  Datacenter: attr('string'),
  DisplayName: attr('string'),
  Kind: attr('string'),
  Namespace: attr('string'),
  AuthURL: attr('string'),
});
