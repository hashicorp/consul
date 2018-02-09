import Controller from '@ember/controller';
import { computed } from '@ember/object';

import put from 'consul-ui/lib/request/put';

export default Controller.extend({
  needs: ["dc", "nodes"],
  dc: computed.alias("controllers.dc"),
  actions: {
    invalidateSession: function(sessionId) {
      var controller = this;
      controller.set('isLoading', true);
      confirm("Are you sure you want to invalidate this session?").then(
        function()
        {
          var node = controller.get('model');
          var dc = controller.get('dc').get('datacenter');
          // Delete the session
          put("/v1/session/destroy/" + sessionId, dc).then(function(response) {
            return get('/v1/session/node/' + node.Node, dc).then(function(data) {
              controller.set('sessions', data);
            });
          }).fail(function(response) {
            // Render the error message on the form if the request failed
            controller.set('errorMessage', 'Received error while processing: ' + response.statusText);
          });
        }
      ).finally(
        function()
        {
          controller.set("isLoading", false);
        }
      )
    }
  }
});
