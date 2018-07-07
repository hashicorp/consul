import Mixin from '@ember/object/mixin';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';

export default Mixin.create({
  _feedback: service('feedback'),
  init: function() {
    this._super(...arguments);
    const feedback = get(this, '_feedback');
    const route = this;
    set(this, 'feedback', {
      execute: function() {
        feedback.execute(...[...arguments, route.controller]);
      },
    });
  },
});
