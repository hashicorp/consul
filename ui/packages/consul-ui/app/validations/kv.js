/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { validatePresence, validateLength } from 'ember-changeset-validations/validators';
export default {
  Key: [validatePresence(true), validateLength({ min: 1 })],
};
