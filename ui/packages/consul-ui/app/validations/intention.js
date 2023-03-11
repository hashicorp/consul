import { validatePresence, validateLength } from 'ember-changeset-validations/validators';
import validateSometimes from 'consul-ui/validations/sometimes';
export default {
  '*': [
    validateSometimes(validatePresence(true), function () {
      const action = this.Action || '';
      const permissions = this.Permissions || [];
      if (action === '' && permissions.length === 0) {
        return true;
      }
      return false;
    }),
  ],
  SourceName: [validatePresence(true), validateLength({ min: 1 })],
  DestinationName: [validatePresence(true), validateLength({ min: 1 })],
  Permissions: [
    validateSometimes(validateLength({ min: 1 }), function (changes, content) {
      return !this.Action;
    }),
  ],
};
