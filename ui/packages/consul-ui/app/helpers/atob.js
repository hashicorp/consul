/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';
import atob from 'consul-ui/utils/atob';
export default helper(function ([str = '']) {
  return atob(str);
});
