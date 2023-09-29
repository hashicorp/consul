/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default {
  access: {
    allow: (item, value) => item.Action === value,
    deny: (item, value) => item.Action === value,
    'app-aware': (item, value) => typeof item.Action === 'undefined',
  },
};
