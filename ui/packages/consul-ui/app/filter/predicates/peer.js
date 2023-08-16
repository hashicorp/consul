/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  state: {
    pending: (item, value) => item.State.toLowerCase() === value,
    establishing: (item, value) => item.State.toLowerCase() === value,
    active: (item, value) => item.State.toLowerCase() === value,
    failing: (item, value) => item.State.toLowerCase() === value,
    terminated: (item, value) => item.State.toLowerCase() === value,
    deleting: (item, value) => item.State.toLowerCase() === value,
  },
};
