import Service from '@ember/service';
import notify from 'consul-ui/utils/notify';
import error from 'consul-ui/utils/error';

export default Service.extend({
  execute: function(handle, successMsg, failureMsg) {
    const controller = this.controller;
    controller.set('isLoading', true);
    return handle()
      .then(function() {
        // this will go into the view
        notify(successMsg);
        controller.set('notification', successMsg);
      })
      .catch(function(e) {
        error(e);
        controller.set('errorMessage', failureMsg);
      })
      .finally(function() {
        controller.set('isLoading', false);
      });
  },
});
