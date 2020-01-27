/*eslint ember/no-observers: "warn"*/
// TODO: Remove ^
import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
import { observer } from '@ember/object';

export default Helper.extend({
  router: service('router'),
  compute([targetRouteName, ...rest]) {
    if (this.router.currentRouteName.startsWith('nspace.') && targetRouteName.startsWith('dc.')) {
      targetRouteName = `nspace.${targetRouteName}`;
    }
    return this.router.isActive(...[targetRouteName, ...rest]);
  },
  onURLChange: observer('router.currentURL', function() {
    this.recompute();
  }),
});
