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
    createNewLabel: function(term) {
      return `Use a future Consul Service called '${term}'`;
    },
    change: function(e, value, _target) {
      // normalize back to standard event
      const target = e.target || { ..._target, ...{ name: e, value: value } };
      switch (target.name) {
        case 'Action':
          set(this.changeset, target.name, target.value);
          break;
        case 'SourceName':
        case 'DestinationName':
          let name = target.value;
          let selected = target.value;
          if (typeof name !== 'string') {
            name = get(target.value, 'Name');
          }
          const match = get(this, 'items').filterBy('Name', name);
          if (match.length === 0) {
            selected = { Name: name };
            const items = [selected].concat(this.items.toArray());
            set(this, 'items', items);
          }
          set(this.changeset, target.name, name);
          set(this, target.name, selected);
          break;
      }
    },
  },
});
