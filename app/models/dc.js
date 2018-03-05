import Entity from 'ember-data/model';
import attr from 'ember-data/attr';
import { hasMany } from 'ember-data/relationships';
export default Entity.extend({
  Name: attr('string'),
  Services: hasMany('service'),
  Nodes: hasMany('node'),
  // probably KV etc
});
