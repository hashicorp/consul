/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';

import rightTrim from 'consul-ui/utils/right-trim';

export default helper(function ([str = '', search = ''], hash) {
  return rightTrim(str, search);
});
