export default function(handle, successMsg, failureMsg) {
  const controller = this.controller;
  controller.set('isLoading', true);
  return handle()
    .then(function() {
      // notify
      console.log(successMsg);
    })
    .catch(function(e) {
      // notify
      console.log(e);
      controller.set('errorMessage', failureMsg);
    })
    .finally(function() {
      controller.set('isLoading', false);
    });
}
