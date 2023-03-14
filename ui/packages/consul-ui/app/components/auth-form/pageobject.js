/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default (submitable, clickable, attribute) =>
  (scope = '.auth-form') => {
    return {
      scope: scope,
      ...submitable(),
    };
  };
