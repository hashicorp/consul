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
      },
    });
  },
  actions: {
    change: function(e) {
      const target = e.target || { name: 'SourceName', value: e };
      switch (target.name) {
        case 'Action':
          set(this.changeset, target.name, target.value);
          break;
        case 'SourceName':
          set(this.changeset, 'SourceName', get(target.value, 'Name'));
          set(this.item, 'SourceName', get(target.value, 'Name'));
          break;
      }
    },
  },
});
