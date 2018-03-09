import Controller, { inject as controller } from '@ember/controller';
import { computed } from '@ember/object';

import { get as getter } from '@ember/object';

import get from 'consul-ui/utils/request/get';
import put from 'consul-ui/utils/request/put';
import del from 'consul-ui/utils/request/del';

export default Controller.extend({
  isLoading: false,
  isLockedOrLoading: computed.or('isLoading', 'isLocked'),
  dc: controller('dc'),
  // TODO: refactor this out
  getParentKeyRoute: function() {
    if (this.get('isRoot')) {
      return this.get('rootKey');
    }
    return this.get('parentKey');
  },
  transitionToNearestParent: function(parent) {
    var controller = this;
    var rootKey = controller.get('rootKey');
    var dc = controller.get('dc'); //.get('datacenter');
    get('/v1/kv/' + parent + '?keys', dc)
      .then(function(data) {
        controller.transitionToRoute('dc.kv.show', parent);
      })
      .fail(function(response) {
        if (response.status === 404) {
          controller.transitionToRoute('dc.kv.show', rootKey);
        }
      });
    controller.set('isLoading', false);
  },
  actions: {
    // Updates the key set as the model on the route.
    updateKey: function() {
      var controller = this;
      controller.set('isLoading', true);
      var dc = this.get('dc'); //.get('datacenter');
      var key = this.get('model');
      // Put the key and the decoded (plain text) value
      // from the form.
      put('/v1/kv/' + getter(key, 'Key'), dc, getter(key, 'valueDecoded'))
        .then(function(response) {
          // If success, just reset the loading state.
          controller.set('isLoading', false);
        })
        .fail(function(response) {
          // Render the error message on the form if the request failed
          controller.set('errorMessage', 'Received error while processing: ' + response.statusText);
        });
    },
    cancelEdit: function() {
      this.set('isLoading', true);
      this.transitionToRoute('dc.kv.show', this.getParentKeyRoute());
      this.set('isLoading', false);
    },
    deleteKey: function() {
      var controller = this;
      controller.set('isLoading', true);
      var dc = controller.get('dc'); //.get('datacenter');
      var key = controller.get('model');
      var parent = controller.getParentKeyRoute();
      // Delete the key
      del('/v1/kv/' + key.get('Key'), dc)
        .then(function(data) {
          controller.transitionToNearestParent(parent);
        })
        .fail(function(response) {
          // Render the error message on the form if the request failed
          controller.set('errorMessage', 'Received error while processing: ' + response.statusText);
        });
    },
  },
});
