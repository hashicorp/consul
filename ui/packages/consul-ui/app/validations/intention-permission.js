import {
  validateInclusion,
  validatePresence,
  validateFormat,
} from 'ember-changeset-validations/validators';
import validateSometimes from 'consul-ui/validations/sometimes';

const name = 'intention-permission';
export default (schema) => ({
  '*': [
    validateSometimes(validatePresence(true), function () {
      const methods = this.get('HTTP.Methods') || [];
      const headers = this.get('HTTP.Header') || [];
      const pathType = this.get('HTTP.PathType') || 'NoPath';
      const path = this.get('HTTP.Path') || '';
      const isValid = [
        methods.length !== 0,
        headers.length !== 0,
        pathType !== 'NoPath' && path !== '',
      ].includes(true);
      return !isValid;
    }),
  ],
  Action: [validateInclusion({ in: schema[name].Action.allowedValues })],
  HTTP: {
    Path: [
      validateSometimes(validateFormat({ regex: /^\// }), function () {
        const pathType = this.get('HTTP.PathType');
        return typeof pathType !== 'undefined' && pathType !== 'NoPath';
      }),
    ],
  },
});
