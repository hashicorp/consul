/*eslint ember/no-observers: "warn"*/
// TODO: Remove ^
import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
import { observes } from '@ember-decorators/object';

export default class IsHrefHelper extends Helper {
  @service('router') router;

  compute([targetRouteName, ...rest]) {
    if (this.router.currentRouteName.startsWith('nspace.') && targetRouteName.startsWith('dc.')) {
      targetRouteName = `nspace.${targetRouteName}`;
    }
    return this.router.isActive(...[targetRouteName, ...rest]);
  }

  @observes('router.currentURL')
  onURLChange() {
    this.recompute();
  }
}
