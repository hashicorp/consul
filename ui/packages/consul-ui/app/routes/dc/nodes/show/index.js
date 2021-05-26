import Route from 'consul-ui/routing/route';
import { get } from '@ember/object';

export default class IndexRoute extends Route {
  afterModel(model, transition) {
    const parent = this.routeName
      .split('.')
      .slice(0, -1)
      .join('.');
    model = this.modelFor(parent);
    // the default selected tab depends on whether you have any healthchecks or not
    // so check the length here.
    const to = get(model, 'item.Checks.length') > 0 ? 'healthchecks' : 'services';
    this.replaceWith(`${parent}.${to}`, model);
  }
}
