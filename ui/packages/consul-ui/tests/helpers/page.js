/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

const mb = function (path) {
  return function (obj) {
    return (
      path.map(function (prop) {
        obj = obj || {};
        if (isNaN(parseInt(prop))) {
          return (obj = obj[prop]);
        } else {
          return (obj = obj.objectAt(parseInt(prop)));
        }
      }) && obj
    );
  };
};
let currentPage;
export const getCurrentPage = function () {
  return currentPage;
};
export const setCurrentPage = function (page) {
  currentPage = page;
  return page;
};
export const find = function (path, page = currentPage) {
  const parts = path.split('.');
  const last = parts.pop();
  let obj;
  let parent = mb(parts)(page) || page;
  if (typeof parent.objectAt === 'function') {
    parent = parent.objectAt(0);
  }
  obj = parent[last];
  if (typeof obj === 'undefined') {
    throw new Error(`PageObject not found: The '${path}' object doesn't exist`);
  }
  if (typeof obj === 'function') {
    obj = obj.bind(parent);
  }
  return obj;
};
