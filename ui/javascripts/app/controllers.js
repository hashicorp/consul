App.DcController = Ember.Controller.extend({
  // Whether or not the dropdown menu can be seen
  isDropdownVisible: false,

  datacenter: function() {
    return this.get('content')
  }.property('Content'),

  checks: function() {
    var nodes = this.get('nodes');
    var checks = Ember.A()

    // Combine the checks from all of our nodes
    // into one.
    nodes.forEach(function(item) {
      checks = checks.concat(item.Checks)
    });

    return checks
  }.property('nodes'),

  // Returns the total number of failing checks.
  //
  // We treat any non-passing checks as failing
  //
  totalChecksFailing: function() {
    var checks = this.get('checks')
    return (checks.filterBy('Status', 'critical').get('length') +
      checks.filterBy('Status', 'warning').get('length'))
  }.property('nodes'),

  //
  // Returns the human formatted message for the button state
  //
  checkMessage: function() {
    var checks = this.get('checks')
    var failingChecks = this.get('totalChecksFailing');
    var passingChecks = checks.filterBy('Status', 'passing').get('length');

    if (this.get('hasFailingChecks') == true) {
      return  failingChecks + ' checks failing';
    } else {
      return  passingChecks + ' checks passing';
    }

  }.property('nodes'),

  //
  // Boolean if the datacenter has any failing checks.
  //
  hasFailingChecks: function() {
    var failingChecks = this.get('totalChecksFailing')
    return (failingChecks > 0);
  }.property('nodes'),

  actions: {
    // Hide and show the dropdown menu
    toggle: function(item){
      this.toggleProperty('isDropdownVisible');
    },
    // Just hide the dropdown menu
    hideDrop: function(item){
      this.set('isDropdownVisible', false);
    }
  }
})

KvBaseController = Ember.ObjectController.extend({
  getParentKeyRoute: function() {
    if (this.get('isRoot')) {
      return this.get('rootKey');
    }
    return this.get("parentKey");
  },

  transitionToNearestParent: function(parent) {
    var controller = this;
    var rootKey = controller.get('rootKey');
    var dc = controller.get('dc').get('datacenter');

    Ember.$.ajax({
      url: ('/v1/kv/' + parent + '?keys&c=' + dc),
      type: 'GET'
    }).then(function(data) {
      controller.transitionToRoute('kv.show', parent);
    }).fail(function(response) {
      if (response.status === 404) {
        controller.transitionToRoute('kv.show', rootKey);
      }
    });

    controller.set('isLoading', false);
  }
});

// Add mixins
App.KvShowController = KvBaseController.extend(Ember.Validations.Mixin);

App.KvShowController.reopen({
  needs: ["dc"],
  dc: Ember.computed.alias("controllers.dc"),
  isLoading: false,

  actions: {
    // Creates the key from the newKey model
    // set on the route.
    createKey: function() {
      this.set('isLoading', true);

      var controller = this;
      var newKey = controller.get('newKey');
      var parentKey = controller.get('parentKey');
      var grandParentKey = controller.get('grandParentKey');
      var dc = controller.get('dc').get('datacenter');

      // If we don't have a previous model to base
      // on our parent, or we're not at the root level,
      // add the prefix
      if (parentKey != undefined && parentKey != "/") {
        newKey.set('Key', (parentKey + newKey.get('Key')));
      }

      // Put the Key and the Value retrieved from the form
      Ember.$.ajax({
          url: ("/v1/kv/" + newKey.get('Key') + '?dc=' + dc),
          type: 'PUT',
          data: newKey.get('Value')
      }).then(function(response) {
        // transition to the right place
        if (newKey.get('isFolder') == true) {
          controller.transitionToRoute('kv.show', newKey.get('Key'));
        } else {
          controller.transitionToRoute('kv.edit', newKey.get('Key'));
        }
        controller.set('isLoading', false)
      }).fail(function(response) {
        // Render the error message on the form if the request failed
        controller.set('errorMessage', 'Received error while processing: ' + response.statusText)
      });
    },

    deleteFolder: function() {
      this.set('isLoading', true);

      var controller = this;
      var grandParent = controller.get('grandParentKey');

      // Delete the folder
      Ember.$.ajax({
          url: ("/v1/kv/" + controller.get('parentKey') + '?recurse'),
          type: 'DELETE'
      }).then(function(response) {
        controller.transitionToNearestParent(grandParent);
      }).fail(function(response) {
        // Render the error message on the form if the request failed
        controller.set('errorMessage', 'Received error while processing: ' + response.statusText)
      });
    }
  }
});

App.KvEditController = KvBaseController.extend({
  isLoading: false,
  needs: ["dc"],
  dc: Ember.computed.alias("controllers.dc"),

  actions: {
    // Updates the key set as the model on the route.
    updateKey: function() {
      this.set('isLoading', true);

      var dc = this.get('dc').get('datacenter');
      var key = this.get("model");
      var controller = this;

      // Put the key and the decoded (plain text) value
      // from the form.
      Ember.$.ajax({
          url: ("/v1/kv/" + key.get('Key') + '?dc=' + dc),
          type: 'PUT',
          data: key.get('valueDecoded')
      }).then(function(response) {
        // If success, just reset the loading state.
        controller.set('isLoading', false)
      }).fail(function(response) {
        // Render the error message on the form if the request failed
        controller.set('errorMessage', 'Received error while processing: ' + response.statusText)
      })
    },

    cancelEdit: function() {
      this.set('isLoading', true);
      this.transitionToRoute('kv.show', this.getParentKeyRoute());
      this.set('isLoading', false);
    },

    deleteKey: function() {
      this.set('isLoading', true);

      var controller = this;
      var dc = controller.get('dc').get('datacenter');
      var key = controller.get("model");
      var parent = controller.getParentKeyRoute();

      // Delete the key
      Ember.$.ajax({
          url: ("/v1/kv/" + key.get('Key') + '?dc=' + dc),
          type: 'DELETE'
      }).then(function(data) {
        controller.transitionToNearestParent(parent);
      }).fail(function(response) {
        // Render the error message on the form if the request failed
        controller.set('errorMessage', 'Received error while processing: ' + response.statusText)
      })
    }
  }

});

App.NodesController = Ember.ArrayController.extend({
  condensedView: true
});
