/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { getContext } from '@ember/test-helpers';
import { getExecutionContext } from 'ember-cli-page-object/-private/execution_context';
import createQueryParams from 'consul-ui/utils/http/create-query-params';

const assign = Object.assign;
const QueryParams = {
  stringify: createQueryParams(),
};

function fillInDynamicSegments(path, params, encoder) {
  return path
    .split('/')
    .map(function (segment) {
      let match = segment.match(/^:(.+)$/);

      if (match) {
        let [, key] = match;
        let value = params[key];

        if (typeof value === 'undefined') {
          throw new Error(`Missing parameter for '${key}'`);
        }

        // Remove dynamic segment key from params
        delete params[key];
        return encoder(value);
      }

      return segment;
    })
    .join('/');
}

function appendQueryParams(path, queryParams) {
  if (Object.keys(queryParams).length > 0) {
    return `${path}?${QueryParams.stringify(queryParams)}`;
  }
  return path;
}
/**
 * Custom implementation of `visitable`
 * Currently aims to be compatible and as close as possible to the
 * actual `ember-cli-page-object` version
 *
 * Additions:
 * 1. Injectable encoder, for when you don't want your segments to be encoded
 *    or you have specific encoding needs
 *    Specifically in my case for KV urls where the `Key`/Slug shouldn't be encoded,
 *    defaults to the browsers `encodeURIComponent` for compatibility and ease.
 * 2. `path` can be an array of (string) paths OR a string for compatibility.
 *    If a path cannot be generated due to a lack of properties on the
 *    dynamic segment params, if will keep trying 'path' in the array
 *    until it finds one that it can construct. This follows the same thinking
 *    as 'if you don't specify an item, then we are looking to create one'
 */
export function visitable(path, encoder = encodeURIComponent) {
  return {
    isDescriptor: true,

    value(dynamicSegmentsAndQueryParams = {}) {
      let executionContext = getExecutionContext(this);

      return executionContext.runAsync((context) => {
        let params;
        let fullPath = (function _try(paths) {
          let path = paths.shift();
          if (typeof dynamicSegmentsAndQueryParams.nspace !== 'undefined') {
            path = `/:nspace${path}`;
          }
          params = assign({}, dynamicSegmentsAndQueryParams);
          let fullPath;
          try {
            fullPath = fillInDynamicSegments(path, params, encoder);
          } catch (e) {
            if (paths.length > 0) {
              fullPath = _try(paths);
            } else {
              throw e;
            }
          }
          return fullPath;
        })(typeof path === 'string' ? [path] : path.slice(0));

        fullPath = appendQueryParams(fullPath, params);

        const container = getContext().owner;
        const locationType = container.lookup('service:env').var('locationType');
        const location = container.lookup(`location:${locationType}`);
        // look for a visit on the current location first before just using
        // visit on the current context/app
        if (typeof location.visit === 'function') {
          return location.visit(fullPath);
        } else {
          return context.visit(fullPath);
        }
      });
    },
  };
}
