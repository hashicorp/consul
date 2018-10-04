import { validatePresence, validateLength } from 'ember-changeset-validations/validators';
export default {
  Policies: validateLength({ min: 1 }),
};
