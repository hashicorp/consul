import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { getOwner } from '@ember/application';
import { get } from '@ember/object';
import transitionable from 'consul-ui/utils/routing/transitionable';

export default Controller.extend({
  router: service('router'),
  http: service('repository/type/event-source'),
  client: service('client/http'),
  store: service('store'),
  feedback: service('feedback'),
  actions: {
    // TODO: We currently do this in the controller instead of the router
    // as the nspace and dc variables aren't available directly on the Route
    // look to see if we can move this up there so we can empty the Controller
    // out again
    reauthorize: function(e) {
      // TODO: For the moment e isn't a real event
      // it has data which is potentially the token
      // and type which is the logout/authorize/use action
      // used for the feedback service.
      this.feedback.execute(
        () => {
          // FIXME: Also need to reset the cache of DataSources
          this.client.abort();
          this.http.resetCache();
          this.store.init();

          const params = {};
          if (e.data) {
            const token = e.data;
            // FIXME: Do I actually need to check to see if nspaces are enabled here?
            if (typeof this.nspace !== 'undefined') {
              const nspace = get(token, 'Namespace') || this.nspace.Name;
              // you potentially have a new namespace
              // if you do redirect to it
              if (nspace !== this.nspace.Name) {
                params.nspace = `~${nspace}`;
              }
            }
          }
          const routeName = this.router.currentRoute.name;
          const route = getOwner(this).lookup(`route:${routeName}`);
          // We use hrefMut here as refresh doesn't work if you are on an error page
          // which is highly likely to happen here (403s)
          if (routeName !== this.router.currentRouteName) {
            return route.transitionTo(...transitionable(this.router, params, getOwner(this)));
          } else {
            return route.refresh();
          }
        },
        e.type,
        function(type, e) {
          return type;
        },
        {}
      );
    },
  },
});
