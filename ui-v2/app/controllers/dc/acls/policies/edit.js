import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import Changeset from 'ember-changeset';
import validations from 'consul-ui/validations/policy';
import lookupValidator from 'ember-changeset-validations';

export default Controller.extend({
  isScoped: false,
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
        case 'Datacenters':
          get(this.changeset, 'Datacenters')[target.checked ? 'pushObject' : 'removeObject'](
            target.value
          );
          break;
        case 'Rules':
          set(this, 'item.Rules', target.value);
          break;
        case 'isScoped':
          set(this, 'isScoped', !get(this, 'isScoped'));
          break;
      }
    },
  },
});
