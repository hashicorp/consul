/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import setHelpers from 'mnemonist/set';

export default {
  status: {
    passing: (item, value) => item.Status === value,
    warning: (item, value) => item.Status === value,
    critical: (item, value) => item.Status === value,
    empty: (item, value) => item.ServiceChecks.length === 0,
  },
  source: (item, values) => {
    return setHelpers.intersectionSize(values, new Set(item.ExternalSources || [])) !== 0;
  },
};
