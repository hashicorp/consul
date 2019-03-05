import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('repository/dc'),
  model: function(params) {
    return hash({
      item: get(this, 'repo').getActive(),
    });
  },
  afterModel: function({ item }, transition) {
    this.transitionTo('dc.services', get(item, 'Name'));
  },
});
