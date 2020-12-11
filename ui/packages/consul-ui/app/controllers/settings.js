import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';

export default Controller.extend({
  dom: service('dom'),
  actions: {
    change: function(e, value, item) {
      const event = this.dom.normalizeEvent(e, value);
      // TODO: Switch to using forms like the rest of the app
      // setting utils/form/builder for things to be done before we
      // can do that. For the moment just do things normally its a simple
      // enough form at the moment

      const target = event.target;
      const blocking = get(this, 'item.client.blocking');
      switch (target.name) {
        case 'client[blocking]':
          set(this, 'item.client.blocking', !blocking);
          this.send('update', 'client', this.item.client);
          break;
      }
    },
  },
});
