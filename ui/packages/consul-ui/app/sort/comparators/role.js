/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default ({ properties }) =>
  (key = 'Name:asc') => {
    return properties(['Name', 'CreateIndex'])(key);
  };
