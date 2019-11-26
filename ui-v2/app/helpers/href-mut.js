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
    let route = this.router.currentRoute.name;
    // TODO: this is specific to consul/nspaces
    // 'ideally' we could try and do this elsewhere
    // not super important though.
    // This will turn an URL that has no nspace (/ui/dc-1/nodes) into one
    // that does have a namespace (/ui/~nspace/nodes) if you href-mut with
    // a nspace parameter
    if (typeof params.nspace !== 'undefined' && !route.startsWith('nspace.')) {
      route = `nspace.${route}`;
      atts.push(params.nspace);
    }
    //
    return hrefTo(this, this.router, [route, ...atts.reverse()], hash);
  },
});
