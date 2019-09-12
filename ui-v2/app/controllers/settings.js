import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';

export default Controller.extend({
  repo: service('settings'),
  dom: service('dom'),
  timeout: service('timeout'),
  confirming: false,
  applyTransition: function() {
    const tick = get(this, 'timeout').tick;
    set(this, 'confirming', true);
    tick().then(() => {
      set(this, 'confirming', false);
    });
  },
  actions: {
    key: function(e) {
      this.applyTransition();
      switch (true) {
        case e.keyCode === 13:
          // disable ENTER
          e.preventDefault();
      }
    },
    change: function(e, value, item) {
      const event = get(this, 'dom').normalizeEvent(e, value);
      // TODO: Switch to using forms like the rest of the app
      // setting utils/form/builder for things to be done before we
      // can do that. For the moment just do things normally its a simple
      // enough form at the moment

      const target = event.target;
      const blocking = get(this, 'item.client.blocking');
      switch (target.name) {
        case 'client[blocking]':
          set(this, 'item.client.blocking', !blocking);
          this.send('update', get(this, 'item'));
          break;
        case 'urls[service]':
          if (typeof get(this, 'item.urls') === 'undefined') {
            set(this, 'item.urls', {});
          }
          set(this, 'item.urls.service', target.value);
          this.send('update', get(this, 'item'));
          break;
      }
    },
  },
});
