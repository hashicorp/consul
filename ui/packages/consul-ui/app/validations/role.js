import { validateFormat } from 'ember-changeset-validations/validators';
export default {
  Name: validateFormat({ regex: /^[A-Za-z0-9\-_]{1,256}$/ }),
};
