import { validatePresence, validateLength } from 'ember-changeset-validations/validators';
export default {
  Key: [validatePresence(true), validateLength({ min: 1 })],
};
