import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { getOwner } from '@ember/application';
import { env } from 'consul-ui/env';
import transitionable from 'consul-ui/utils/routing/transitionable';

const DEFAULT_NSPACE_PARAM = '~default';
export default Route.extend({
  repo: service('repository/dc'),
  router: service('router'),
  // The ember router seems to change the priority of individual routes
  // depending on whether they are wildcard routes or not.
  // This means that the namespace routes will be recognized before kv ones
  // even though we define namespace routes after kv routes (kv routes are
  // wildcard routes)
  // Therefore here whenever we detect that ember has recognized a nspace route
  // when it shouldn't (we know this as there is no ~ in the nspace param)
  // we recalculate the route it should have caught by generating the nspace
  // equivalent route for the url (/dc-1/kv/services > /~default/dc-1/kv/services)
  // and getting the information for that route. We then remove the nspace specific
  // information that we generated onto the route, which leaves us with the route
  // we actually want. Using this final route information we redirect the user
  // to where they wanted to go.
  beforeModel: function(transition) {
    if (!this.paramsFor('nspace').nspace.startsWith('~')) {
      const url = `${env('rootURL')}${DEFAULT_NSPACE_PARAM}${transition.intent.url}`;
      const route = this.router.recognize(url);
      const [name, ...params] = transitionable(route, {}, getOwner(this));
      this.replaceWith.apply(this, [
        // remove the 'nspace.' from the routeName
        name
          .split('.')
          .slice(1)
          .join('.'),
        // remove the nspace param from the params
        ...params.slice(1),
      ]);
    }
  },
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
