import Controller from '@ember/controller';
import { computed } from '@ember/object';

import get from 'consul-ui/lib/request/get';
import put from 'consul-ui/lib/request/put';
import del from 'consul-ui/lib/request/del';
import confirm from 'consul-ui/lib/confirm';

export default Controller.extend({
        getParentKeyRoute: function() {
            if (this.get('isRoot')) {
                return this.get('rootKey');
            }
            return this.get('parentKey');
        },
        transitionToNearestParent: function(parent) {
            var controller = this;
            var rootKey = controller.get('rootKey');
            var dc = controller.get('dc').get('datacenter');
            get('/v1/kv/' + parent + '?keys', dc).then(function(data) {
                controller.transitionToRoute('kv.show', parent);
            }).fail(function(response) {
                if (response.status === 404) {
                    controller.transitionToRoute('kv.show', rootKey);
                }
            });
            controller.set('isLoading', false);
        },
  //
        needs: ["dc"],
        // dc: computed.alias("controllers.dc"),
        isLoading: false,
        actions: {
            // Creates the key from the newKey model
            // set on the route.
            createKey: function() {
                var controller = this;
                controller.set('isLoading', true);
                var newKey = controller.get('newKey');
                var parentKey = controller.get('parentKey');
                var grandParentKey = controller.get('grandParentKey');
                var dc = controller.get('dc');//.get('datacenter');
                // If we don't have a previous model to base
                // on our parent, or we're not at the root level,
                // add the prefix
                if (parentKey !== undefined && parentKey !== "/") {
                    newKey.set('Key', (parentKey + newKey.get('Key')));
                }
                // Put the Key and the Value retrieved from the form
                put("/v1/kv/" + newKey.get('Key'), dc, newKey.get("Value")).then(function(response) {
                    // transition to the right place
                    if (newKey.get('isFolder') === true) {
                        controller.transitionToRoute('kv.show', newKey.get('Key'));
                    } else {
                        controller.transitionToRoute('kv.edit', newKey.get('Key'));
                    }
                    controller.set('isLoading', false);
                }).fail(function(response) {
                    // Render the error message on the form if the request failed
                    controller.set('errorMessage', 'Received error while processing: ' + response.statusText);
                });
            },
            deleteFolder: function() {
                var controller = this;
                controller.set('isLoading', true);
                var dc = controller.get('dc').get('datacenter');
                var grandParent = controller.get('grandParentKey');
                confirm("Are you sure you want to delete this folder?").then(
                    function()
                    {
                        // Delete the folder
                        del("/v1/kv/" + controller.get('parentKey') + '?recurse', dc).then(function(response) {
                            controller.transitionToNearestParent(grandParent);
                        }).fail(function(response) {
                            // Render the error message on the form if the request failed
                            controller.set('errorMessage', 'Received error while processing: ' + response.statusText);
                        });
                    }
                ).finally(
                    function()
                    {
                        controller.set('isLoading', true);
                    }
                )
            }
        }
});
