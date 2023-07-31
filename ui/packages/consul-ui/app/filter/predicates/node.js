/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default {
  status: {
    passing: (item, value) => item.Status === value,
    warning: (item, value) => item.Status === value,
    critical: (item, value) => item.Status === value,
  },
  version: (item, value) => item.Version.includes(value + '.'),
};
