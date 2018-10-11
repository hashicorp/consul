import Route from '@ember/routing/route';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';

export default Route.extend({
  settings: service('settings'),
  repo: service('tokens'),
  actions: {
    authorize: function(token) {
      const dc = this.modelFor('dc').dc.Name;
      get(this, 'repo')
        .self(token, dc)
        // TODO: What happens when there is an error?
        .then(item => {
          get(this, 'settings')
            .persist({
              token: {
                AccessorID: get(item, 'AccessorID'),
                SecretID: token,
              },
            })
            .then(() => {
              this.refresh();
            });
        });
    },
  },
});
