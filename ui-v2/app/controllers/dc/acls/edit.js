import Controller from '@ember/controller';
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
});
