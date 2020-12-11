import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default class IndexRoute extends Route {
  @service('repository/dc')
  repo;

  model(params) {
    return hash({
      item: this.repo.getActive(),
    });
  }

  afterModel({ item }, transition) {
    this.transitionTo('dc.services', get(item, 'Name'));
  }
}
