import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { hasMany } from 'ember-data/relationships';
export default Model.extend({
  Name: attr('string'),
  Services: hasMany('service'),
  Nodes: hasMany('node'),
  // probably KV etc
});
