import Component from '@ember/component';
import { set } from '@ember/object';
import { inject as service } from '@ember/service';

export default Component.extend({
  service: service('state'),
  tagName: '',
  didReceiveAttrs: function() {
    if (typeof this.state === 'undefined') {
      return;
    }
    let match = true;
    if (typeof this.matches !== 'undefined') {
      match = this.service.matches(this.state, this.matches);
    }
    set(this, 'rendering', match);
  },
});
