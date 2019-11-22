import { validateFormat } from 'ember-changeset-validations/validators';
export default {
  Name: validateFormat({ regex: /^[A-Za-z0-9\-]{1,64}$/ }),
};
