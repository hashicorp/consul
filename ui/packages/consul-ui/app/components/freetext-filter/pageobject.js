/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default (triggerable) => () => {
  return {
    ...{
      search: triggerable('keypress', '[name="s"]'),
    },
  };
};
