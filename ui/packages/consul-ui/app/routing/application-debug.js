import ApplicationRoute from '../routes/application';

let isDebugRoute = false;
const routeChange = function(transition) {
  isDebugRoute = transition.to.name.startsWith('docs');
};

export default class DebugRoute extends ApplicationRoute {
  constructor(owner) {
    super(...arguments);
    this.router = owner.lookup('service:router');
    this.router.on('routeWillChange', routeChange);
  }

  renderTemplate() {
    if (isDebugRoute) {
      this.render('debug');
    } else {
      super.renderTemplate(...arguments);
    }
  }

  willDestroy() {
    this.router.off('routeWillChange', routeChange);
    super.willDestroy(...arguments);
  }
}
