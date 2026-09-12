/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default ({ properties }) =>
  (key = 'Path:asc') => {
    return properties(['Path'])(key);
  };
