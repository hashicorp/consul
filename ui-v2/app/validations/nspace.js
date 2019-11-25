import { validateFormat } from 'ember-changeset-validations/validators';
export default {
  Name: validateFormat({ regex: /^[a-zA-Z0-9]([a-zA-Z0-9-]{0,62}[a-zA-Z0-9])?$/ }),
};
