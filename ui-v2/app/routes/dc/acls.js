import Route from '@ember/routing/route';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';

export default Route.extend({
  settings: service('settings'),
  // repo:
  actions: {
    authorize: function(token) {
      get(this, 'settings')
        .persist({
          token: token,
        })
        .then(() => {
          this.refresh();
        });
    },
  },
});
