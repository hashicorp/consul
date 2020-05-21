import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { getOwner } from '@ember/application';
import { get } from '@ember/object';
import transitionable from 'consul-ui/utils/routing/transitionable';

export default Controller.extend({
  router: service('router'),
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
          // TODO: Currently we clear cache from the ember-data store
          // ideally this would be a static method of the abstract Repository class
          // once we move to proper classes for services take another look at this.
          this.store.clear();
          //
          const params = {};
          if (e.data) {
            const token = e.data;
            // TODO: Do I actually need to check to see if nspaces are enabled here?
            if (typeof this.nspace !== 'undefined') {
              const nspace = get(token, 'Namespace') || this.nspace.Name;
              // you potentially have a new namespace
              // if you do redirect to it
              if (nspace !== this.nspace.Name) {
                params.nspace = `~${nspace}`;
              }
            }
          }
          const container = getOwner(this);
          const routeName = this.router.currentRoute.name;
          const route = container.lookup(`route:${routeName}`);
          // Refresh the application route
          return container
            .lookup('route:application')
            .refresh()
            .promise.then(res => {
              // Use transitionable if we need to change a section of the URL
              if (
                routeName !== this.router.currentRouteName ||
                typeof params.nspace !== 'undefined'
              ) {
                return route.transitionTo(
                  ...transitionable(this.router.currentRoute, params, container)
                );
              } else {
                return res;
              }
            });
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
