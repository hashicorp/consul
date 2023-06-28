/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

/**
 * basic path.resolve like function for resolving ember Route-type paths
 * importantly your from should look ember-y route-like (i.e. with no prefix /)
 * and your to should begin with either ./ or ../
 * if to begins with a / then ../ and ./ in the to are not currently
 * resolved
 */
export default (from, to) => {
  if (to.indexOf('/') === 0) {
    return to;
  }
  return to
    .split('/')
    .reduce((prev, item, i, items) => {
      if (item !== '.') {
        if (item === '..') {
          prev.pop();
        } else if (item !== '' || i === items.length - 1) {
          prev.push(item);
        }
      }
      return prev;
    }, from.split('/'))
    .join('/');
};
