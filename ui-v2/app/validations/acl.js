import { validatePresence, validateLength } from 'ember-changeset-validations/validators';
export default {
  Name: [validatePresence(true), validateLength({ min: 10 })],
  Type: validatePresence(true),
  Rules: [validatePresence(true), validateLength({ min: 1 })],
  ID: validateLength({ min: 1 }),
};
