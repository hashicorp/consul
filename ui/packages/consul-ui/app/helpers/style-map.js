/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

/**
 * Conditionally maps styles to a string ready for typical DOM
 * usage (i.e. semi-colon delimited)
 *
 * @typedef {([string, (string | undefined), string] | [string, (string | undefined)])} styleInfo
 * @param {styleInfo[]} entries - An array of `styleInfo`s to map
 * @param {boolean} transform=true - whether to perform the build-time 'helper
 *  to modifier' transpilation. Note a transpiler needs installing separately.
 */
const styleMap = (entries, transform = true) => {
  const str = entries.reduce((prev, [prop, value, unit = '']) => {
    if (value == null) {
      return prev;
    }
    return `${prev}${prop}:${value.toString()}${unit};`;
  }, '');
  return str.length > 0 ? str : undefined;
};

export default helper(styleMap);
