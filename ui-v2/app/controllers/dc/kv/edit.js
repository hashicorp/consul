import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';

import Changeset from 'ember-changeset';
import validations from 'consul-ui/validations/kv';
import lookupValidator from 'ember-changeset-validations';
export default Controller.extend({
  json: true,
  encoder: service('btoa'),
  setProperties: function(model) {
    // TODO: Potentially save whether json has been clicked to the model,
    // setting set(this, 'json', true) here will force the form to always default to code=on
    // even if the user has selected code=off on another KV
    // ideally we would save the value per KV, but I'd like to not do that on the model
    // a set(this, 'json', valueFromSomeStorageJustForThisKV) would be added here
    this.changeset = new Changeset(model.item, lookupValidator(validations), validations);
    this._super({
      ...model,
      ...{
        item: this.changeset,
      },
    });
  },
  actions: {
    change: function(e) {
      const target = e.target || { name: 'value', value: e };
      var parent;
      switch (target.name) {
        case 'additional':
          parent = get(this, 'parent.Key');
          set(this.changeset, 'Key', `${parent !== '/' ? parent : ''}${target.value}`);
          break;
        case 'json':
          set(this, 'json', !get(this, 'json'));
          break;
        case 'value':
          set(this, 'item.Value', get(this, 'encoder').execute(target.value));
          break;
      }
    },
  },
});
