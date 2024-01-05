/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { validateFormat } from 'ember-changeset-validations/validators';
export default {
  Name: validateFormat({ regex: /^[A-Za-z0-9\-_]{1,128}$/ }),
};
