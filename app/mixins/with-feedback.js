import Mixin from '@ember/object/mixin';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';

export default Mixin.create({
  feedback: service('feedback'),
  init: function() {
    this._super(...arguments);
    set(this, 'feedback', {
      execute: get(this, 'feedback').execute.bind(this),
    });
  },
});
