import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
import { getOwner } from '@ember/application';

export default Helper.extend({
  router: service('router'),
  compute(params, hash) {
    return () => {
      const container = getOwner(this);
      const routeName = this.router.currentRoute.name;
      return container.lookup(`route:${routeName}`).refresh();
    };
  },
});
