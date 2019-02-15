import Mixin from '@ember/object/mixin';

export default Mixin.create({
  reset: function(exiting) {
    if (exiting) {
      Object.keys(this).forEach(prop => {
        if (this[prop] && typeof this[prop].close === 'function') {
          this[prop].close();
          // ember doesn't delete on 'resetController' by default
          delete this[prop];
        }
      });
    }
    return this._super(...arguments);
  },
});
