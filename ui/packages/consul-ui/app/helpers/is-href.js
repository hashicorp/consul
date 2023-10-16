import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class IsHrefHelper extends Helper {
  @service('router') router;
  init() {
    super.init(...arguments);
    this.router.on('routeWillChange', this.routeWillChange);
  }

  compute([targetRouteName, ...rest]) {
    if (this.router.currentRouteName.startsWith('nspace.') && targetRouteName.startsWith('dc.')) {
      targetRouteName = `nspace.${targetRouteName}`;
    }
    if (typeof this.next !== 'undefined' && this.next !== 'loading') {
      return this.next.startsWith(targetRouteName);
    }
    return this.router.isActive(...[targetRouteName, ...rest]);
  }

  @action
  routeWillChange(transition) {
    this.next = transition.to.name.replace('.index', '');
    this.recompute();
  }

  willDestroy() {
    this.router.off('routeWillChange', this.routeWillChange);
    super.willDestroy();
  }
}
