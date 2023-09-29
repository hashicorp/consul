/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default {
  kind: {
    folder: (item, value) => item.isFolder,
    key: (item, value) => !item.isFolder,
  },
};
