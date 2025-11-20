/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { visit as emberVisit, getContext } from '@ember/test-helpers';
import action from 'ember-cli-page-object/-private/action';
import createQueryParams from 'consul-ui/utils/http/create-query-params';

const qpStringify = createQueryParams();

function fillInDynamicSegments(path, params, encoder) {
  return path
    .split('/')
    .map(function (segment) {
      const match = segment.match(/^:(.+)$/);
      if (match) {
        const [, key] = match;
        const value = params[key];
        if (value === undefined) {
          throw new Error(`Missing parameter for '${key}'`);
        }
        delete params[key];
        return encoder(value);
      }
      return segment;
    })
    .join('/');
}

function appendQueryParams(path, queryParams) {
  const keys = Object.keys(queryParams);
  return keys.length > 0 ? `${path}?${qpStringify(queryParams)}` : path;
}

/**
 * Custom implementation of `visitable` for Consul UI
 * 
 * Enhanced version based on ember-cli-page-object v2.3.2
 * 
 * Custom features:
 * 1. Injectable encoder - customize dynamic segment encoding (for KV URLs, etc.)
 * 2. Multiple path templates - automatic fallback when segments are missing
 * 3. Namespace injection - auto-prepends `/:nspace` segment when needed
 * 4. Custom location service - integrates with Consul's routing system
 
* 1. Injectable encoder, for when you don't want your segments to be encoded
 *    or you have specific encoding needs
 *    Specifically in my case for KV urls where the `Key`/Slug shouldn't be encoded,
 *    defaults to the browsers `encodeURIComponent` for compatibility and ease.
 * 2. `path` can be an array of (string) paths OR a string for compatibility.
 *    If a path cannot be generated due to a lack of properties on the
 *    dynamic segment params, if will keep trying 'path' in the array
 *    until it finds one that it can construct. This follows the same thinking
 *    as 'if you don't specify an item, then we are looking to create one'
 
 * @param {string|string[]} path - Single path or array of path templates
 * @param {Function} encoder - Encoding function (default: encodeURIComponent)
 * @return {Descriptor}
 */
export function visitable(path, encoder = encodeURIComponent) {
  return action(function (dynamicSegmentsAndQueryParams = {}) {
    const params = { ...dynamicSegmentsAndQueryParams };
    
    // Try multiple path templates if provided as array
    const paths = Array.isArray(path) ? path.slice() : [path];
    let fullPath;
    
    for (const template of paths) {
      const pathWithNs = params.nspace !== undefined ? `/:nspace${template}` : template;
      const paramsCopy = { ...params };
      
      try {
        fullPath = fillInDynamicSegments(pathWithNs, paramsCopy, encoder);
        // Sync consumed params
        Object.keys(params).forEach(key => {
          if (!(key in paramsCopy)) delete params[key];
        });
        break;
      } catch (e) {
        if (template === paths[paths.length - 1]) throw e;
      }
    }
    
    fullPath = appendQueryParams(fullPath, params);
    
    // Use custom location service if available
    const { owner } = getContext();
    const locationType = owner.lookup('service:env').var('locationType');
    const location = owner.lookup(`location:${locationType}`);
    
    if (location && typeof location.visit === 'function') {
      return location.visit(fullPath).catch((e) => {
        throw new Error(`Failed to visit URL '${fullPath}': ${e.toString()}`, { cause: e });
      });
    }
    
    return emberVisit(fullPath).catch((e) => {
      throw new Error(`Failed to visit URL '${fullPath}': ${e.toString()}`, { cause: e });
    });
  });
}