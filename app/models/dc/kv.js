// import Model from 'ember-data';
import Model, { computed, get } from '@ember/object';

export default Model.extend({

  // Validates using the Ember.Validations library
  validations: {
    Key: { presence: true }
  },
  // Boolean if field should validate JSON
  validateJson: false,
  // Boolean if the key is valid
  keyValid: Ember.computed.empty('errors.Key'),
  // Boolean if the value is valid
  valueValid: Ember.computed.empty('errors.Value'),
  // The key with the parent removed.
  // This is only for display purposes, and used for
  // showing the key name inside of a nested key.
  keyWithoutParent: function() {
    return (this.get('Key').replace(this.get('parentKey'), ''));
  }.property('Key'),
  // Boolean if the key is a "folder" or not, i.e is a nested key
  // that feels like a folder. Used for UI
  isFolder: function() {
    if (this.get('Key') === undefined) {
      return false;
    }
    return (this.get('Key').slice(-1) === '/');
  }.property('Key'),
  // Boolean if the key is locked or now
  isLocked: function() {
    if (!this.get('Session')) {
      return false;
    } else {
      return true;
    }
  }.property('Session'),
  // Determines what route to link to. If it's a folder,
  // it will link to kv.show. Otherwise, kv.edit
  linkToRoute: function() {
    if (this.get('Key').slice(-1) === '/') {
      return 'dc.kv.show';
    } else {
      return 'dc.kv.edit';
    }
  }.property('Key'),
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
      return "";
    }
    if (Base64.extendString) {
      // you have to explicitly extend String.prototype
      Base64.extendString();
    }
    // base64 decode the value
    return (this.get('Value').fromBase64());
  }.property('Value'),
  // Check if JSON is valid by attempting a native JSON parse
  isValidJson: function() {
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
  }.property('Value'),
  // An array of the key broken up by the /
  keyParts: function() {
    var key = this.get('Key');
    // If the key is a folder, remove the last
    // slash to split properly
    if (key.slice(-1) == "/") {
      key = key.substring(0, key.length - 1);
    }
    return key.split('/');
  }.property('Key'),
  // The parent Key is the key one level above this.Key
  // key: baz/bar/foobar/
  // grandParent: baz/bar/
  parentKey: function() {
    var parts = this.get('keyParts').toArray();
    // Remove the last item, essentially going up a level
    // in hiearchy
    parts.pop();
    return parts.join("/") + "/";
  }.property('Key'),
  // The grandParent Key is the key two levels above this.Key
  // key: baz/bar/foobar/
  // grandParent: baz/
  grandParentKey: function() {
    var parts = this.get('keyParts').toArray();
    // Remove the last two items, jumping two levels back
    parts.pop();
    parts.pop();
    return parts.join("/") + "/";
  }.property('Key')
});
