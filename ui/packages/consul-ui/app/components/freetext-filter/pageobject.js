/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (triggerable) => () => {
  return {
    ...{
      search: triggerable('keypress', '[name="s"]'),
    },
  };
};
