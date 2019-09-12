// This helper requires `ember-href-to` for the moment at least
// It's similar code but allows us to check on the type of route
// (dynamic or wildcard) and encode or not depending on the type
import Helper from '@ember/component/helper';
import { hrefTo } from 'ember-href-to/helpers/href-to';

import wildcard from 'consul-ui/utils/routing/wildcard';

import { routes } from 'consul-ui/router';

const isWildcard = wildcard(routes);

export default Helper.extend({
  compute([targetRouteName, ...rest], namedArgs) {
    if (isWildcard(targetRouteName)) {
      rest = rest.map(function(item, i) {
        return item
          .split('/')
          .map(encodeURIComponent)
          .join('/');
      });
    }
    if (namedArgs.params) {
      return hrefTo(this, namedArgs.params);
    } else {
      return hrefTo(this, [targetRouteName, ...rest]);
    }
  },
});
