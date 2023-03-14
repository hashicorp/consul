/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default ({ properties }) =>
  (key = 'MethodName:asc') => {
    return properties(['MethodName', 'TokenTTL'])(key);
  };
