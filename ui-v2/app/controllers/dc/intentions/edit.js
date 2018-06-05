import Controller from '@ember/controller';
import { get, set } from '@ember/object';
// import Changeset from 'ember-changeset';
// import validations from 'consul-ui/validations/acl';
// import lookupValidator from 'ember-changeset-validations';

export default Controller.extend({
  setProperties: function(model) {
    this.changeset = model.item; //new Changeset(model.item, lookupValidator(validations), validations);
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
      const target = e.target || { ..._target, ...{ name: e, value: value } };
      switch (target.name) {
        case 'Action':
          set(this.changeset, target.name, target.value);
          break;
        case 'SourceName':
        case 'DestinationName':
          set(this.changeset, target.name, get(target.value, 'Name'));
          set(this.item, target.name, get(target.value, 'Name'));
          set(this, target.name, target.value);
          break;
      }
    },
  },
});
