/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (encode = encodeURIComponent) {
  return function stringify(obj, parent) {
    return Object.entries(obj)
      .reduce(function (prev, [key, value], i) {
        // if the value is undefined do nothing
        if (typeof value === 'undefined') {
          return prev;
        }
        let prop = encode(key);
        // if we have a parent, prefix the property with that
        if (typeof parent !== 'undefined') {
          prop = `${parent}[${prop}]`;
        }
        // if the value is null just print the prop
        if (value === null) {
          return prev.concat(prop);
        }
        // anything nested, recur
        if (typeof value === 'object') {
          return prev.concat(stringify(value, prop));
        }
        // anything else print prop=value
        return prev.concat(`${prop}=${encode(value)}`);
      }, [])
      .join('&');
  };
}
