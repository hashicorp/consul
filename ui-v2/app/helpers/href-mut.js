import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
import { hrefTo } from 'consul-ui/helpers/href-to';

const getRouteParams = function(route, params = {}) {
  return route.paramNames.map(function(item) {
    if (typeof params[item] !== 'undefined') {
      return params[item];
    }
    return route.params[item];
  });
};
export default Helper.extend({
  router: service('router'),
  compute([params], hash) {
    let current = this.router.currentRoute;
    let parent;
    let atts = getRouteParams(current, params);
    // walk up the entire route/s replacing any instances
    // of the specified params with the values specified
    while ((parent = current.parent)) {
      atts = atts.concat(getRouteParams(parent, params));
      current = parent;
    }
    return hrefTo(this, this.router, [this.router.currentRoute.name, ...atts.reverse()], hash);
  },
});
