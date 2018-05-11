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

export function visitable(path, encoder = encodeURIComponent) {
  return {
    isDescriptor: true,

    value(dynamicSegmentsAndQueryParams = {}) {
      let executionContext = getExecutionContext(this);

      return executionContext.runAsync(context => {
        let params = assign({}, dynamicSegmentsAndQueryParams);
        let fullPath = fillInDynamicSegments(path, params, encoder);

        fullPath = appendQueryParams(fullPath, params);

        return context.visit(fullPath);
      });
    },
  };
}
