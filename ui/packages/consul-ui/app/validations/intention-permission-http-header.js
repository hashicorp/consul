import { validatePresence } from 'ember-changeset-validations/validators';
import validateSometimes from 'ember-changeset-conditional-validations/validators/sometimes';
export default (schema) => ({
  Name: [validatePresence(true)],
  Value: validateSometimes([validatePresence(true)], function () {
    return this.get('HeaderType') !== 'Present';
  }),
});
