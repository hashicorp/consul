import Model from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed, get } from '@ember/object';
import ascend from 'consul-ui/utils/ascend';
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

  basename: computed('Key', function() {
    let key = get(this, 'Key') || '';
    if (isFolder(key)) {
      key = key.substring(0, key.length - 1);
    }
    return (get(this, 'Key') || '').replace(ascend(key || '', 1) || '/', '');
  }),
  isFolder: computed('Key', function() {
    return isFolder(get(this, 'Key') || '');
  }),
  // Boolean if the key is locked or now
  isLocked: computed('Session', function() {
    // handlebars doesn't like booleans, use valueOf
    return new Boolean(get(this, 'Session')).valueOf();
  }),
});
