/* global Base64 */
import Entity from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed } from '@ember/object';
import ascend from 'consul-ui/utils/ascend';
import isFolder from 'consul-ui/utils/isFolder';

export default Entity.extend({
  Key: attr('string'),
  LockIndex: attr('number'),
  Flags: attr('number'),
  Value: attr('string'),
  CreateIndex: attr('string'),
  ModifyIndex: attr('string'),
  Session: attr('string'), // probably belongsTo

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
    let key = this.get('Key');
    if (isFolder(key)) {
      key = key.substring(0, key.length - 1);
    }
    return this.get('Key').replace(ascend(key, 1) || '/', '');
  }),
  isFolder: computed('Key', function() {
    return isFolder(this.get('Key'));
  }),
  // Boolean if the key is locked or now
  isLocked: computed('Session', function() {
    // handlebars doesn't like booleans, use valueOf
    return new Boolean(this.get('Session')).valueOf();
  }),
  // Determines what route to link to. If it's a folder,
  // it will link to kv.show. Otherwise, kv.edit
  linkToRoute: computed('Key', function() {
    if (isFolder(this.get('Key'))) {
      return 'dc.kv.show';
    } else {
      return 'dc.kv.edit';
    }
  }),
  // The base64 decoded value of the key.
  // if you set on this key, it will update
  // the key.Value
  valueDecoded: function(key, value) {
    // setter
    if (arguments.length > 1) {
      this.set('Value', value);
      return value;
    }
    // getter
    // If the value is null, we don't
    // want to try and base64 decode it, so just return
    if (this.get('Value') == null) {
      return '';
    }
    if (Base64.extendString) {
      // you have to explicitly extend String.prototype
      Base64.extendString();
    }
    // base64 decode the value
    return this.get('Value').fromBase64();
  }.property('Value'),
  // Check if JSON is valid by attempting a native JSON parse
  isValidJson: computed('Value', function() {
    var value;
    try {
      window.atob(this.get('Value'));
      value = this.get('valueDecoded');
    } catch (e) {
      value = this.get('Value');
    }
    try {
      JSON.parse(value);
      return true;
    } catch (e) {
      return false;
    }
  }),
});
