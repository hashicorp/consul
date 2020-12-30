/*eslint ember/no-observers: "warn"*/
// TODO: Remove ^
// This helper requires `ember-href-to` for the moment at least
// It's similar code but allows us to check on the type of route
// (dynamic or wildcard) and encode or not depending on the type
import { inject as service } from '@ember/service';
import Helper from '@ember/component/helper';
import { observes } from '@ember-decorators/object';
import { hrefTo as _hrefTo } from 'ember-href-to/helpers/href-to';

import wildcard from 'consul-ui/utils/routing/wildcard';

import { routes } from 'consul-ui/router';

const isWildcard = wildcard(routes);
export const hrefTo = function(owned, router, [targetRouteName, ...rest], namedArgs) {
  if (isWildcard(targetRouteName)) {
    rest = rest.map(function(item, i) {
      return item
        .split('/')
        .map(encodeURIComponent)
        .join('/');
    });
  }
  if (namedArgs.params) {
    return _hrefTo(owned, namedArgs.params);
  } else {
    // we don't check to see if nspaces are enabled here as routes
    // with beginning with 'nspace' only exist if nspaces are enabled

    // this globally converts non-nspaced href-to's to nspace aware
    // href-to's only if you are within a namespace
    const currentRouteName = router.currentRouteName || '';
    if (currentRouteName.startsWith('nspace.') && targetRouteName.startsWith('dc.')) {
      targetRouteName = `nspace.${targetRouteName}`;
    }
    return _hrefTo(owned, [targetRouteName, ...rest]);
  }
};

export default class HrefToHelper extends Helper {
  @service('router') router;

  compute(params, hash) {
    return hrefTo(this, this.router, params, hash);
  }

  @observes('router.currentURL')
  onURLChange() {
    this.recompute();
  }
}
