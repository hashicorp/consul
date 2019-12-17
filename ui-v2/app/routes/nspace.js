import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('repository/dc'),
  router: service('router'),
  model: function(params) {
    return hash({
      item: this.repo.getActive(),
      nspace: params.nspace,
    });
  },
  afterModel: function(params) {
    // We need to redirect if someone doesn't specify
    // the section they want, but not redirect if the 'section' is
    // specified (i.e. /dc-1/ vs /dc-1/services)
    // check how many parts are in the URL to figure this out
    // if there is a better way to do this then would be good to change
    if (this.router.currentURL.split('/').length < 4) {
      if (!params.nspace.startsWith('~')) {
        this.transitionTo('dc.services', params.nspace);
      } else {
        this.transitionTo('nspace.dc.services', params.nspace, params.item.Name);
      }
    }
  },
});
