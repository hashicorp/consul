/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

export default helper(function flattenProperty([obj, prop], hash) {
  const pages = hash.pages || [];
  pages.push(...obj.pages);
  obj.children.forEach((child) => flattenProperty([child], { pages: pages }));
  return pages;
});
