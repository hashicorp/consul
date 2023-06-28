/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default ({ properties }) =>
  (key) => {
    return properties(['CreateTime'])(key);
  };
