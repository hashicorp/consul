/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (submitable, clickable, attribute) =>
  (scope = '.auth-form') => {
    return {
      scope: scope,
      ...submitable(),
    };
  };
