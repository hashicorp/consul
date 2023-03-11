/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { collection } from 'ember-cli-page-object';

export default (scope = '.consul-intention-permission-list') => {
  return {
    scope: scope,
    intentionPermissions: collection('[data-test-list-row]', {}),
  };
};
