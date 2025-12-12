/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  kind: {
    'global-management': (item, value) => item.isGlobalManagement,
    global: (item, value) => !item.Local,
    local: (item, value) => item.Local,
  },
};
