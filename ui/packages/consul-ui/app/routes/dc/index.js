import Route from 'consul-ui/routing/route';

export default class IndexRoute extends Route {
  beforeModel() {
    this.transitionTo('dc.services');
  }
}
