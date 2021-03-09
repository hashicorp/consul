import { get } from '@ember/object';
export default function(routes) {
  return function(name) {
    let wildcard = false;
    try {
      const path = `route.${name.split('.').join('.route.')}`;
      wildcard = get(routes, path).path.indexOf('*') !== -1;
    } catch (e) {
      // passthrough
    }
    return wildcard;
  };
}
