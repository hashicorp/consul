/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

const filter = function (routeName, atts, params) {
  return [routeName, ...atts];
};
const replaceRouteParams = function (route, params = {}) {
  return (route.paramNames || [])
    .map(function (item) {
      if (typeof params[item] !== 'undefined') {
        return params[item];
      }
      return route.params[item];
    })
    .reverse();
};
export default function (route, params = {}, container) {
  if (route === null) {
    route = container.lookup('route:application');
  }
  let atts = replaceRouteParams(route, params);
  // walk up the entire route/s replacing any instances
  // of the specified params with the values specified
  let current = route;
  let parent;
  while ((parent = current.parent)) {
    atts = atts.concat(replaceRouteParams(parent, params));
    current = parent;
  }
  // Reverse atts here so it doesn't get confusing whilst debugging
  // (.reverse is destructive)
  atts.reverse();
  return filter(route.name || 'application', atts, params);
}
