import Component from '@ember/component';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';

export default Component.extend({
  tagName: '',
  encoder: service('btoa'),
  json: true,
  ondelete: function () {
    this.onsubmit(...arguments);
  },
  oncancel: function () {
    this.onsubmit(...arguments);
  },
  onsubmit: function () {},
  actions: {
    change: function (e, form) {
      const item = form.getData();
      try {
        form.handleEvent(e);
      } catch (err) {
        const target = e.target;
        let parent;
        switch (target.name) {
          case 'value':
            set(item, 'Value', this.encoder.execute(target.value));
            break;
          case 'additional':
            parent = get(this, 'parent');
            set(item, 'Key', `${parent !== '/' ? parent : ''}${target.value}`);
            break;
          case 'json':
            // TODO: Potentially save whether json has been clicked to the model,
            // setting set(this, 'json', true) here will force the form to always default to code=on
            // even if the user has selected code=off on another KV
            // ideally we would save the value per KV, but I'd like to not do that on the model
            // a set(this, 'json', valueFromSomeStorageJustForThisKV) would be added here
            set(this, 'json', !this.json);
            break;
          default:
            throw err;
        }
      }
    },
  },
});
