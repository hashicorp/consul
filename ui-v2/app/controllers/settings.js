import Controller from '@ember/controller';
import confirm from 'consul-ui/utils/confirm';
import error from 'consul-ui/utils/error';
export default Controller.extend({
  actions: {
    requestDelete: function(item) {
      confirm('Are you sure you want to reset your settings?')
        .then(confirmed => {
          if (confirmed) {
            return this.send('delete', item);
          }
        })
        .catch(error);
    },
    change: function(e) {
      this.send('update', { token: e.target.value });
    },
    close: function() {
      this.transitionToRoute('index');
    },
  },
});
