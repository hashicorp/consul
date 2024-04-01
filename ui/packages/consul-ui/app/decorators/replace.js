/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

/**
 * Simple replacing decorator, with the primary usecase for avoiding null API
 * errors by decorating model attributes: @replace(null, []) @attr() Tags;
 */
export const replace = (find, replace) => (target, propertyKey, desc) => {
  return {
    get: function () {
      const value = desc.get.apply(this, arguments);
      if (value === find) {
        return replace;
      }
      return value;
    },
    set: function () {
      return desc.set.apply(this, arguments);
    },
  };
};
export const nullValue = function (val) {
  return replace(null, val);
};
export default replace;
