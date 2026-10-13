/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  behavior: {
    release: (item, value) => item.Behavior === value,
    delete: (item, value) => item.Behavior === value,
  },
};
