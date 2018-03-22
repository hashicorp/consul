import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed } from '@ember/object';

export default Model.extend({
  ID: attr('string'),
  Name: attr('string'),
  Type: attr('string'),
  Rules: attr('string'),
  CreateIndex: attr('number'),
  ModifyIndex: attr('number'),
  Datacenter: attr('string'),
  filterKey: computed.alias('Name'),
  isNotAnon: computed('ID', function() {
    if (this.get('ID') === 'anonymous') {
      return false;
    } else {
      return true;
    }
  }),
});
