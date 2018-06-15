import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import Changeset from 'ember-changeset';
import lookupValidator from 'ember-changeset-validations';

import validations from 'consul-ui/validations/intention';

export default Controller.extend({
  setProperties: function(model) {
    this.changeset = new Changeset(model.item, lookupValidator(validations), validations);
    const sourceName = get(model.item, 'SourceName');
    const destinationName = get(model.item, 'DestinationName');
    let source = model.items.findBy('Name', sourceName);
    let destination = model.items.findBy('Name', destinationName);
    if (!source) {
      source = { Name: sourceName };
      model.items = [source].concat(model.items);
    }
    if (!destination) {
      destination = { Name: destinationName };
      model.items = [destination].concat(model.items);
    }
    this._super({
      ...model,
      ...{
        item: this.changeset,
        SourceName: source,
        DestinationName: destination,
      },
    });
  },
  actions: {
    createNewLabel: function(term) {
      return `Use a future Consul Service called '${term}'`;
    },
    isUnique: function(term) {
      return !get(this, 'items').findBy('Name', term);
    },
    change: function(e, value, _target) {
      // normalize back to standard event
      const target = e.target || { ..._target, ...{ name: e, value: value } };
      let name, selected;
      name = selected = target.value;
      // TODO:
      // linter needs this here?
      let match;
      switch (target.name) {
        case 'Description':
        case 'Action':
          set(this.changeset, target.name, target.value);
          break;
        case 'SourceName':
        case 'DestinationName':
          if (typeof name !== 'string') {
            name = get(target.value, 'Name');
          }
          // linter doesn't like const here
          match = get(this, 'items').filterBy('Name', name);
          if (match.length === 0) {
            selected = { Name: name };
            // linter doesn't mind const here?
            const items = [selected].concat(this.items.toArray());
            set(this, 'items', items);
          }
          set(this.changeset, target.name, name);
          set(this, target.name, selected);
          break;
      }
      this.changeset.validate();
    },
  },
});
