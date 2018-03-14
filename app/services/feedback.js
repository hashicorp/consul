import Service from '@ember/service';

export default Service.extend({
  execute: function(handle, successMsg, failureMsg) {
    const controller = this.controller;
    controller.set('isLoading', true);
    return handle()
      .then(function() {
        // notify
        console.log(successMsg);
        controller.set('notification', successMsg);
      })
      .catch(function(e) {
        // notify
        console.error(e);
        controller.set('errorMessage', failureMsg);
      })
      .finally(function() {
        controller.set('isLoading', false);
      });
  },
});
