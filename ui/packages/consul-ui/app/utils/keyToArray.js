/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/**
 * Turns a separated path or 'key name' in this case to
 * an array. If the key name is simply the separator (for example '/')
 * then the array should contain a single empty string value
 *
 * @param {String} key - The separated path/key
 * @param {String} separator - The separator
 * @returns {String[]}
 */
export default function (key, separator = '/') {
  return (key === separator ? '' : key).split(separator);
}
