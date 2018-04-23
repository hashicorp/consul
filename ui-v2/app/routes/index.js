import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('dc'),
  model: function(params) {
    return hash({
      item: get(this, 'repo').getActive(),
    });
  },
  afterModel: function({ item }, transition) {
    this.transitionTo('dc.services', get(item, 'Name'));
  },
});
