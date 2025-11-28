/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

export default helper(function serviceHealthPercentage([params] /*, hash*/) {
  if (!params || Object.keys(params).length === 0) {
    return '';
  }
  const total = params.ChecksCritical + params.ChecksPassing + params.ChecksWarning;

  if (total === 0) {
    return '';
  } else {
    return {
      passing: Math.round((params.ChecksPassing / total) * 100),
      warning: Math.round((params.ChecksWarning / total) * 100),
      critical: Math.round((params.ChecksCritical / total) * 100),
    };
  }
});
