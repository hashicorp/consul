import Entity from 'ember-data/model';
import attr from 'ember-data/attr';
import { computed } from '@ember/object';

export default Entity.extend({
  Key: attr('string'),
  LockIndex: attr('number'),
  Flags: attr('number'),
  Value: attr('string'),
  CreateIndex: attr('string'),
  ModifyIndex: attr('string'),

  // Validates using the Ember.Validations library
  validations: {
    Key: { presence: true },
  },
  // Boolean if field should validate JSON
  validateJson: false,
  // Boolean if the key is valid
  keyValid: computed.empty('errors.Key'),
  // Boolean if the value is valid
  valueValid: computed.empty('errors.Value'),
  // The key with the parent removed.
  // This is only for display purposes, and used for
  // showing the key name inside of a nested key.
  keyWithoutParent: function() {
    return this.get('Key').replace(this.get('parentKey'), '');
  }.property('Key'),
  // Boolean if the key is a "folder" or not, i.e is a nested key
  // that feels like a folder. Used for UI
  isFolder: computed('Key', function() {
    if (this.get('Key') === undefined) {
      return false;
    }
    return this.get('Key').slice(-1) === '/';
  }),
  // Boolean if the key is locked or now
  isLocked: computed('Session', function() {
    if (!this.get('Session')) {
      return false;
    } else {
      return true;
    }
  }),
  // Determines what route to link to. If it's a folder,
  // it will link to kv.show. Otherwise, kv.edit
  linkToRoute: computed('Key', function() {
    if (this.get('Key').slice(-1) === '/') {
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
  // An array of the key broken up by the /
  keyParts: computed('Key', function() {
    var key = this.get('Key');
    // If the key is a folder, remove the last
    // slash to split properly
    if (key.slice(-1) == '/') {
      key = key.substring(0, key.length - 1);
    }
    return key.split('/');
  }),
  // The parent Key is the key one level above this.Key
  // key: baz/bar/foobar/
  // grandParent: baz/bar/
  parentKey: computed('Key', function() {
    var parts = this.get('keyParts').toArray();
    // Remove the last item, essentially going up a level
    // in hiearchy
    parts.pop();
    return parts.join('/') + '/';
  }),
  // The grandParent Key is the key two levels above this.Key
  // key: baz/bar/foobar/
  // grandParent: baz/
  grandParentKey: computed('Key', function() {
    var parts = this.get('keyParts').toArray();
    // Remove the last two items, jumping two levels back
    parts.pop();
    parts.pop();
    return parts.join('/') + '/';
  }),
});
