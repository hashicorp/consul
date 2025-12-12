/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default ({ properties }) =>
  (key = 'MethodName:asc') => {
    return properties(['MethodName', 'TokenTTL'])(key);
  };
