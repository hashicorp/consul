//
// A Consul service.
//
App.Service = Ember.Object.extend({
  //
  // The number of failing checks within the service.
  //
  failingChecks: function() {
    return this.get('Checks').filterBy('Status', 'critical').get('length');
  }.property('Checks'),

  //
  // The number of passing checks within the service.
  //
  passingChecks: function() {
    return this.get('Checks').filterBy('Status', 'passing').get('length');
  }.property('Checks'),

  //
  // The formatted message returned for the user which represents the
  // number of checks failing or passing. Returns `1 passing` or `2 failing`
  //
  checkMessage: function() {
    if (this.get('hasFailingChecks') === false) {
      return this.get('passingChecks') + ' passing';
    } else {
      return this.get('failingChecks') + ' failing';
    }
  }.property('Checks'),

  //
  // Boolean of whether or not there are failing checks in the service.
  // This is used to set color backgrounds and so on.
  //
  hasFailingChecks: function() {
    return (this.get('failingChecks') > 0);
  }.property('Checks')
});

//
// A Consul Node
//
App.Node = Ember.Object.extend({
  //
  // The number of failing checks within the service.
  //
  failingChecks: function() {
    return this.get('Checks').filterBy('Status', 'critical').get('length');
  }.property('Checks'),

  //
  // The number of passing checks within the service.
  //
  passingChecks: function() {
    return this.get('Checks').filterBy('Status', 'passing').get('length');
  }.property('Checks'),

  //
  // The formatted message returned for the user which represents the
  // number of checks failing or passing. Returns `1 passing` or `2 failing`
  //
  checkMessage: function() {
    if (this.get('hasFailingChecks') === false) {
      return this.get('passingChecks') + ' passing';
    } else {
      return this.get('failingChecks') + ' failing';
    }
  }.property('Checks'),

  //
  // Boolean of whether or not there are failing checks in the service.
  // This is used to set color backgrounds and so on.
  //
  hasFailingChecks: function() {
    return (this.get('failingChecks') > 0);
  }.property('Checks')
});


//
// A key/value object
//
App.Key = Ember.Object.extend(Ember.Validations.Mixin, {
  validations: {
    key: { presence: true },
    value: { presence: true }
  },

  keyValid: Ember.computed.empty('errors.key'),
  valueValid: Ember.computed.empty('errors.value'),

  isFolder: function() {
    return (this.get('key').slice(-1) == "/")
  }.property('key'),

  urlSafeKey: function() {
    return this.get('key').replace(/\//g, "-")
  }.property('key'),

  linkToRoute: function() {
    var key = this.get('urlSafeKey')

    if (key.slice(-1) === "-") {
      return 'kv.show'
    } else {
      return 'kv.edit'
    }
  }.property('key'),

  keyParts: function() {
    var key = this.get('key');

    if (key.slice(-1) == "/") {
      key = key.substring(0, key.length - 1);
    }
    return key.split('/');
  }.property('key'),

  parentKey: function() {
    var parts = this.get('keyParts').toArray();

    parts.pop();

    return parts.join("/") + "/";
  }.property('key'),

  grandParentKey: function() {
    var parts = this.get('keyParts').toArray();

    parts.pop();
    parts.pop();

    return parts.join("/") + "/";
  }.property('key')
});
