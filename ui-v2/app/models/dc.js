import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { hasMany } from 'ember-data/relationships';

export const PRIMARY_KEY = 'uid';
export const FOREIGN_KEY = 'Datacenter';
export const SLUG_KEY = 'Name';

export default Model.extend({
  [PRIMARY_KEY]: attr('string'),
  [SLUG_KEY]: attr('string'),
  // TODO: Are these required?
  Services: hasMany('service'),
  Nodes: hasMany('node'),
});
