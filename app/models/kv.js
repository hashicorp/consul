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

  // Validates using the Ember.Validations library
  // validations: {
  //   Key: { presence: true },
  // },
  // Boolean if field should validate JSON
  validateJson: false,
  // Boolean if the key is valid
  keyValid: computed.empty('errors.Key'),
  // Boolean if the value is valid
  valueValid: computed.empty('errors.Value'),
  // The key with the parent removed.
  // This is only for display purposes, and used for
  // showing the key name inside of a nested key.
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
  // Determines what route to link to. If it's a folder,
  // it will link to kv.show. Otherwise, kv.edit
  linkToRoute: computed('Key', function() {
    if (isFolder(get(this, 'Key'))) {
      return 'dc.kv.index';
    } else {
      return 'dc.kv.edit';
    }
  }),
  // Check if JSON is valid by attempting a native JSON parse
  isValidJson: computed('Value', function() {
    var value;
    try {
      window.atob(get(this, 'Value'));
      value = get(this, 'valueDecoded');
    } catch (e) {
      value = get(this, 'Value');
    }
    try {
      JSON.parse(value);
      return true;
    } catch (e) {
      return false;
    }
  }),
});
