/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

// TODO: Currently I'm only using this for hardcoded values
// so ' ' to '-' replacement is sufficient for the moment
export function slugify([str = ''], hash) {
  return str.replace(/ /g, '-').toLowerCase();
}

export default helper(slugify);
