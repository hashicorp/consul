App.DcController = Ember.Controller.extend({
  // Whether or not the dropdown menu can be seen
  isDropdownVisible: false,

  checks: function() {
    var nodes = this.get('nodes');
    var checks = Ember.A()

    // Combine the checks from all of our nodes
    // into one.
    nodes.forEach(function(item) {
      checks = checks.concat(item.Checks)
    });

    return checks
  }.property('Checks'),

  // Returns the total number of failing checks.
  //
  // We treat any non-passing checks as failing
  //
  totalChecksFailing: function() {
    var checks = this.get('checks')
    return (checks.filterBy('Status', 'critical').get('length') +
      checks.filterBy('Status', 'warning').get('length'))
  }.property('Checks'),

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

  }.property('Checks'),

  //
  // Boolean if the datacenter has any failing checks.
  //
  hasFailingChecks: function() {
    var checks = this.get('checks')
    return (checks.filterBy('Status', 'critical').get('length') > 0);
  }.property('Checks'),

  actions: {
    // Hide and show the dropdown menu
    toggle: function(item){
      this.toggleProperty('isDropdownVisible');
    }
  }
})

// Add mixins
App.KvShowController = Ember.ObjectController.extend(Ember.Validations.Mixin);

App.KvShowController.reopen({
  isLoading: false,

  actions: {
    // Creates the key from the newKey model
    // set on the route.
    createKey: function() {
      this.set('isLoading', true);

      var newKey = this.get('newKey');
      var parentKey = this.get('parentKey');
      var grandParentKey = this.get('grandParentKey');
      var controller = this;

      // If we don't have a previous model to base
      // on our parent, or we're not at the root level,
      // strip the leading slash.
      if (!parentKey || parentKey != "/") {
        newKey.set('Key', (parentKey + newKey.get('Key')));
      }

      // Put the Key and the Value retrieved from the form
      Ember.$.ajax({
          url: "/v1/kv/" + newKey.get('Key'),
          type: 'PUT',
          data: newKey.get('Value')
      }).then(function(response) {
        controller.set('isLoading', false)
        // Transition to edit the key
        controller.transitionToRoute('kv.edit', newKey.get('urlSafeKey'));
        // Reload the keys in the left column
        controller.get('keys').reload()
      }).fail(function(response) {
        // Render the error message on the form if the request failed
        controller.set('errorMessage', 'Received error while processing: ' + response.statusText)
      });

    }
  }
});

App.KvEditController = Ember.Controller.extend({
  isLoading: false,

  actions: {
    // Updates the key set as the model on the route.
    updateKey: function() {
      this.set('isLoading', true);

      var key = this.get("model");
      var controller = this;

      // Put the key and the decoded (plain text) value
      // from the form.
      Ember.$.ajax({
          url: "/v1/kv/" + key.get('Key'),
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

    deleteKey: function() {
      this.set('isLoading', true);

      var key = this.get("model");
      var controller = this;
      // Get the parent for the transition back up a level
      // after the delete
      var parent = key.get('urlSafeParentKey');

      // Delete the key
      Ember.$.ajax({
          url: "/v1/kv/" + key.get('Key'),
          type: 'DELETE'
      }).then(function(response) {
        controller.set('isLoading', false);
        // Tranisiton back up a level
        controller.transitionToRoute('kv.show', parent);
      }).fail(function(response) {
        // Render the error message on the form if the request failed
        controller.set('errorMessage', 'Received error while processing: ' + response.statusText)
      })

    }
  }

});
