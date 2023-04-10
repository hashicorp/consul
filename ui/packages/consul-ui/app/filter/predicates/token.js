/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default {
  kind: {
    'global-management': (item, value) => item.isGlobalManagement,
    global: (item, value) => !item.Local,
    local: (item, value) => item.Local,
  },
};
