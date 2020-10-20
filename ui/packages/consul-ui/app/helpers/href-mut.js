import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
import { hrefTo } from 'consul-ui/helpers/href-to';
import { getOwner } from '@ember/application';
import transitionable from 'consul-ui/utils/routing/transitionable';

export default Helper.extend({
  router: service('router'),
  compute([params], hash) {
    return hrefTo(
      this,
      this.router,
      transitionable(this.router.currentRoute, params, getOwner(this)),
      hash
    );
  },
});
