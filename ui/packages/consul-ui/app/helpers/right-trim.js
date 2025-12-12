/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

import rightTrim from 'consul-ui/utils/right-trim';

export default helper(function ([str = '', search = ''], hash) {
  return rightTrim(str, search);
});
