// This helper requires `ember-href-to` for the moment at least
// It's similar code but allows us to check on the type of route
// (dynamic or wildcard) and encode or not depending on the type
import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { getOwner } from '@ember/application';

import transitionable from 'consul-ui/utils/routing/transitionable';
import wildcard from 'consul-ui/utils/routing/wildcard';
import { routes } from 'consul-ui/router';

const isWildcard = wildcard(routes);

export const hrefTo = function (container, params, hash = {}) {
  // TODO: consider getting this from @service('router')._router which is
  // private but we don't need getOwner, and it ensures setupRouter is called
  // How private is 'router:main'? If its less private maybe stick with it?
  const location = container.lookup('router:main').location;
  const router = container.lookup('service:router');

  let _params = params.slice(0);
  let routeName = _params.shift();
  let _hash = hash.params || {};
  // a period means use the same routeName we are currently at and therefore
  // use transitionable to figure out all the missing params
  if (routeName === '.') {
    _params = transitionable(router.currentRoute, _hash, container);
    // _hash = {};
    routeName = _params.shift();
  }

  try {
    // if the routeName is a wildcard (*) route url encode all of the params
    if (isWildcard(routeName)) {
      _params = _params.map((item, i) => {
        return item.split('/').map(encodeURIComponent).join('/');
      });
    }
    return location.hrefTo(routeName, _params, _hash);
  } catch (e) {
    if (e.constructor === Error) {
      e.message = `${e.message} For "${params[0]}:${JSON.stringify(params.slice(1))}"`;
    }
    throw e;
  }
};

export default class HrefToHelper extends Helper {
  @service('router') router;

  init() {
    super.init(...arguments);
    this.router.on('routeWillChange', this.routeWillChange);
  }

  compute(params, hash) {
    return hrefTo(getOwner(this), params, hash);
  }

  @action
  routeWillChange(transition) {
    this.recompute();
  }

  willDestroy() {
    this.router.off('routeWillChange', this.routeWillChange);
    super.willDestroy();
  }
}
