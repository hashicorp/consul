import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';

import Changeset from 'ember-changeset';
import validations from 'consul-ui/validations/kv';
import lookupValidator from 'ember-changeset-validations';
export default Controller.extend({
  json: false,
  encoder: service('btoa'),
  setProperties: function(model) {
    // TODO: Potentially save whether json has been clicked to the model
    set(this, 'json', false);
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
