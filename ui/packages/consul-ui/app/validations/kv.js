/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { validatePresence, validateLength } from 'ember-changeset-validations/validators';
export default {
  Key: [validatePresence(true), validateLength({ min: 1 })],
};
