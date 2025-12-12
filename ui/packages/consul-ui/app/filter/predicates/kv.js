/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default {
  kind: {
    folder: (item, value) => item.isFolder,
    key: (item, value) => !item.isFolder,
  },
};
