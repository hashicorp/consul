import Service, { inject as service } from '@ember/service';

import lookupValidator from 'ember-changeset-validations';
import { Changeset as createChangeset } from 'ember-changeset';

import Changeset from 'consul-ui/utils/form/changeset';

import intentionPermissionValidator from 'consul-ui/validations/intention-permission';
import intentionPermissionHttpHeaderValidator from 'consul-ui/validations/intention-permission-http-header';

const validators = {
  'intention-permission': intentionPermissionValidator,
  'intention-permission-http-header': intentionPermissionHttpHeaderValidator,
};

export default Service.extend({
  schema: service('schema'),

  init: function() {
    this._super(...arguments);
    this._validators = new Map();
  },
  willDestroy: function() {
    this._validators = null;
  },
  changesetFor: function(modelName, model, options = {}) {
    const validator = this.validatorFor(modelName, options);
    let changeset;
    if (validator) {
      let validatorFunc = validator;
      if (typeof validator !== 'function') {
        validatorFunc = lookupValidator(validator);
      }
      changeset = createChangeset(model, validatorFunc, validator, { changeset: Changeset });
    } else {
      changeset = createChangeset(model);
    }
    return changeset;
  },
  validatorFor: function(modelName, options = {}) {
    if (!this._validators.has(modelName)) {
      const factory = validators[modelName];
      let validator;
      if (typeof factory !== 'undefined') {
        validator = factory(this.schema);
      }
      this._validators.set(modelName, validator);
    }
    return this._validators.get(modelName);
  },
});
