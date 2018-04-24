import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed, get } from '@ember/object';
import isFolder from 'consul-ui/utils/isFolder';

export default Model.extend({
  Key: attr('string'),
  LockIndex: attr('number'),
  Flags: attr('number'),
  Value: attr('string'),
  CreateIndex: attr('string'),
  ModifyIndex: attr('string'),
  Session: attr('string'), // probably belongsTo
  Datacenter: attr('string'),

  isFolder: computed('Key', function() {
    return isFolder(get(this, 'Key') || '');
  }),
});
