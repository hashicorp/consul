/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { validatePresence } from 'ember-changeset-validations/validators';
import validateSometimes from 'consul-ui/validations/sometimes';
export default (schema) => ({
  Name: [validatePresence(true)],
  Value: [
    validateSometimes(validatePresence(true), function () {
      return this.get('HeaderType') !== 'Present';
    }),
  ],
});
