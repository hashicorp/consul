/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  status: {
    passing: (item, value) => item.Status === value,
    warning: (item, value) => item.Status === value,
    critical: (item, value) => item.Status === value,
  },
};
