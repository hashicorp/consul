/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import rightTrim from 'consul-ui/utils/right-trim';
export default {
  Key: (item) =>
    rightTrim(item.Key.toLowerCase())
      .split('/')
      .filter((item) => Boolean(item))
      .pop(),
};
