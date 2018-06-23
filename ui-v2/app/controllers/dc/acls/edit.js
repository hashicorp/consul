import Controller from '@ember/controller';
import { set } from '@ember/object';
import Changeset from 'ember-changeset';
import validations from 'consul-ui/validations/acl';
import lookupValidator from 'ember-changeset-validations';

export default Controller.extend({
  setProperties: function(model) {
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
      const target = e.target || { name: 'Rules', value: e };
      switch (target.name) {
        case 'Type':
          set(this.changeset, target.name, target.value);
          break;
        case 'Rules':
          set(this, 'item.Rules', target.value);
          break;
      }
    },
  },
});
