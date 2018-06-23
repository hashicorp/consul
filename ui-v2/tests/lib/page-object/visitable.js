// import { assign } from '../-private/helpers';
const assign = Object.assign;
import { getExecutionContext } from 'ember-cli-page-object/-private/execution_context';

import $ from '-jquery';

function fillInDynamicSegments(path, params, encoder) {
  return path
    .split('/')
    .map(function(segment) {
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
  if (Object.keys(queryParams).length) {
    path += `?${$.param(queryParams)}`;
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

      return executionContext.runAsync(context => {
        var params;
        let fullPath = (function _try(paths) {
          const path = paths.shift();
          params = assign({}, dynamicSegmentsAndQueryParams);
          var fullPath;
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

        return context.visit(fullPath);
      });
    },
  };
}
