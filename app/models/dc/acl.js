// import Model from 'ember-data';
import Model, { computed, get } from '@ember/object';

export default Model.extend({
  isNotAnon: function() {
    if (this.get('ID') === 'anonymous') {
      return false;
    } else {
      return true;
    }
  }.property('ID'),
});
