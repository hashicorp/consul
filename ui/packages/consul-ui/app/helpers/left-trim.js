/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';
import leftTrim from 'consul-ui/utils/left-trim';

export default helper(function ([str = '', search = ''], hash) {
  return leftTrim(str, search);
});
