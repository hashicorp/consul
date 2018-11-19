import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';

export default Controller.extend({
  dom: service('dom'),
  builder: service('form'),
  encoder: service('btoa'),
  json: true,
  init: function() {
    this._super(...arguments);
    this.form = get(this, 'builder').form('kv');
  },
  setProperties: function(model) {
    // essentially this replaces the data with changesets
    this._super(
      Object.keys(model).reduce((prev, key, i) => {
        switch (key) {
          case 'item':
            prev[key] = this.form.setData(prev[key]).getData();
            break;
        }
        return prev;
      }, model)
    );
  },
  actions: {
    change: function(e, value, item) {
      const event = get(this, 'dom').normalizeEvent(e, value);
      const form = get(this, 'form');
      try {
        form.handleEvent(event);
      } catch (err) {
        const target = event.target;
        let parent;
        switch (target.name) {
          case 'value':
            set(this.item, 'Value', get(this, 'encoder').execute(target.value));
            break;
          case 'additional':
            parent = get(this, 'parent.Key');
            set(this.item, 'Key', `${parent !== '/' ? parent : ''}${target.value}`);
            break;
          case 'json':
            // TODO: Potentially save whether json has been clicked to the model,
            // setting set(this, 'json', true) here will force the form to always default to code=on
            // even if the user has selected code=off on another KV
            // ideally we would save the value per KV, but I'd like to not do that on the model
            // a set(this, 'json', valueFromSomeStorageJustForThisKV) would be added here
            set(this, 'json', !get(this, 'json'));
            break;
          default:
            throw err;
        }
      }
    },
  },
});
