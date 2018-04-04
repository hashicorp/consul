import Mixin from '@ember/object/mixin';
import { inject as service } from '@ember/service';

export default Mixin.create({
  feedback: service('feedback'),
  init: function() {
    this._super(...arguments);
    this.set('feedback', {
      execute: this.get('feedback').execute.bind(this),
    });
  },
});
