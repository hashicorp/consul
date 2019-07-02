import { get } from '@ember/object';
import { assert } from '@ember/debug';

export default function(routes) {
  return function(id) {
    const queryParams = get(routes, `${id}._options.query`);
    assert(`Route ${id} doesn't exist`, queryParams);
    return queryParams;
  };
}
