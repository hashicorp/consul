import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
import { hrefTo } from 'consul-ui/helpers/href-to';
import { getOwner } from '@ember/application';

const getRouteParams = function(route, params = {}) {
  return (route.paramNames || []).map(function(item) {
    if (typeof params[item] !== 'undefined') {
      return params[item];
    }
    return route.params[item];
  });
};
export default Helper.extend({
  router: service('router'),
  compute([params], hash) {
    let currentRoute = this.router.currentRoute;
    if (currentRoute === null) {
      currentRoute = getOwner(this).lookup('route:application');
    }
    let parent;
    let atts = getRouteParams(currentRoute, params);
    // walk up the entire route/s replacing any instances
    // of the specified params with the values specified
    let current = currentRoute;
    while ((parent = current.parent)) {
      atts = atts.concat(getRouteParams(parent, params));
      current = parent;
    }
    let route = currentRoute.name || 'application';
    // TODO: this is specific to consul/nspaces
    // 'ideally' we could try and do this elsewhere
    // not super important though.
    // This will turn an URL that has no nspace (/ui/dc-1/nodes) into one
    // that does have a namespace (/ui/~nspace/dc-1/nodes) if you href-mut with
    // a nspace parameter
    if (typeof params.nspace !== 'undefined' && route.startsWith('dc.')) {
      route = `nspace.${route}`;
      atts.push(params.nspace);
    }
    //
    return hrefTo(this, this.router, [route, ...atts.reverse()], hash);
  },
});
