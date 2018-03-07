import Model from '@ember/object';

export default Model.extend({
  isNotAnon: function() {
    if (this.get('ID') === 'anonymous') {
      return false;
    } else {
      return true;
    }
  }.property('ID'),
});
