import { validatePresence, validateLength } from 'ember-changeset-validations/validators';
export default {
  SourceName: [validatePresence(true), validateLength({ min: 1 })],
  DestinationName: [validatePresence(true), validateLength({ min: 1 })],
  Action: validatePresence(true),
};
