/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (submitable, clickable, attribute) =>
  (scope = '.auth-form') => {
    return {
      scope: scope,
      ...submitable(),
    };
  };
