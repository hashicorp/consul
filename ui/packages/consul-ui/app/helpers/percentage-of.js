/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

export default helper(function ([of, num], hash) {
  const perc = (of / num) * 100;
  if (isNaN(perc)) {
    return 0;
  }
  return perc.toFixed(2);
});
