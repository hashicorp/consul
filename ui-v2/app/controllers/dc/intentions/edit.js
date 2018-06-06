import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import Changeset from 'ember-changeset';
import lookupValidator from 'ember-changeset-validations';

import validations from 'consul-ui/validations/intention';

export default Controller.extend({
  setProperties: function(model) {
    this.changeset = new Changeset(model.item, lookupValidator(validations), validations);
    this._super({
      ...model,
      ...{
        item: this.changeset,
        SourceName: model.items.filterBy('Name', get(model.item, 'SourceName'))[0],
        DestinationName: model.items.filterBy('Name', get(model.item, 'DestinationName'))[0],
      },
    });
  },
  actions: {
    change: function(e, value, _target) {
      // normalize back to standard event
      const target = e.target || { ..._target, ...{ name: e, value: value } };
      switch (target.name) {
        case 'Action':
          set(this.changeset, target.name, target.value);
          console.log(target.name, target.value, get(this.changeset, target.name));
          break;
        case 'SourceName':
        case 'DestinationName':
          set(this.changeset, target.name, get(target.value, 'Name'));
          set(this, target.name, target.value);
          break;
      }
    },
  },
});
