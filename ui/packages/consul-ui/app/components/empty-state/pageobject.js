/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default (present) =>
  (scope = '.empty-state') => {
    return {
      scope: scope,
      login: present('[data-test-empty-state-login]'),
    };
  };
