import Model from 'ember-data/model';
import attr from 'ember-data/attr';

export default Model.extend({
  ID: attr('string'),
  Name: attr('string'),
  Type: attr('string'),
  Rules: attr('string'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
  Datacenter: attr('string'),
});
