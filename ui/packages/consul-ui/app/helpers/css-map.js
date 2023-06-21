/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { helper } from '@ember/component/helper';
import { CSSResult } from '@lit/reactive-element';

/**
 * Conditionally maps cssInfos to an array ready for ShadowDom::styles
 * usage.
 *
 * @typedef {([CSSResult, boolean] | [CSSResult])} cssInfo
 * @param {(cssInfo | string)[]} entries - An array of 'entry-like' arrays of `cssInfo`s to map
 */
const cssMap = (entries) => {
  return entries
    .filter((entry) => (entry instanceof CSSResult ? true : entry[entry.length - 1]))
    .map((entry) => (entry instanceof CSSResult ? entry : entry[0]));
};
export default helper(cssMap);
