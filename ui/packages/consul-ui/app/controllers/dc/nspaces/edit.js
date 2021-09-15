import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
export default Controller.extend({
  dom: service('dom'),
  builder: service('form'),
  init: function() {
    this._super(...arguments);
    this.form = this.builder.form('nspace');
  },
  actions: {
    change: function(e, value, item) {
      const event = this.dom.normalizeEvent(e, value);
      try {
        this.form.handleEvent(event);
      } catch (err) {
        const target = event.target;
        switch (target.name) {
          default:
            throw err;
        }
      }
    },
  },
});
