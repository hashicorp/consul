/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (present) =>
  (scope = '.empty-state') => {
    return {
      scope: scope,
      login: present('[data-test-empty-state-login]'),
    };
  };
