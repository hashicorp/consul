// This helper requires `ember-href-to` for the moment at least
// It's similar code but allows us to check on the type of route
// (dynamic or wildcard) and encode or not depending on the type
import Helper from '@ember/component/helper';
import { hrefTo } from 'ember-href-to/helpers/href-to';
import urlEncode from 'consul-ui/utils/url-encode';
import { routes } from 'consul-ui/router';
import { get } from '@ember/object';
const encode = urlEncode(encodeURIComponent);
export default Helper.extend({
  compute([targetRouteName, ...rest], namedArgs) {
    if (namedArgs.params) {
      return hrefTo(this, ...namedArgs.params);
    } else {
      const isWildcard = get(routes, targetRouteName)._options.path.indexOf('*') !== -1;
      if (isWildcard) {
        return hrefTo(this, targetRouteName, ...encode(rest));
      } else {
        return hrefTo(this, targetRouteName, ...rest);
      }
    }
  },
});
