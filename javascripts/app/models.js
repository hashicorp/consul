//
// A Consul service.
//
App.Service = Ember.Object.extend({
  //
  // The number of failing checks within the service.
  //
  failingChecks: function() {
    if (this.get('ChecksCritical') != undefined) {
      return (this.get('ChecksCritical') + this.get('ChecksWarning'))
    } else {
      return this.get('Checks').filterBy('Status', 'critical').get('length');
    }
  }.property('Checks'),

  //
  // The number of passing checks within the service.
  //
  passingChecks: function() {
    if (this.get('ChecksPassing') != undefined) {
      return this.get('ChecksPassing')
    } else {
      return this.get('Checks').filterBy('Status', 'passing').get('length');
    }
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
    Key: { presence: true },
    Value: { presence: true }
  },

  keyValid: Ember.computed.empty('errors.Key'),
  valueValid: Ember.computed.empty('errors.Value'),

  keyWithoutParent: function() {
    return (this.get('Key').replace(this.get('parentKey'), ''));
  }.property('Key'),

  isFolder: function() {
    return (this.get('Key').slice(-1) == "/")
  }.property('Key'),

  urlSafeKey: function() {
    console.log(this)

    return this.get('Key').replace(/\//g, "-")
  }.property('Key'),

  linkToRoute: function() {
    var key = this.get('urlSafeKey')

    if (key.slice(-1) === "-") {
      return 'kv.show'
    } else {
      return 'kv.edit'
    }
  }.property('Key'),

  keyParts: function() {
    var key = this.get('Key');

    if (key.slice(-1) == "/") {
      key = key.substring(0, key.length - 1);
    }
    return key.split('/');
  }.property('Key'),

  parentKey: function() {
    var parts = this.get('keyParts').toArray();

    parts.pop();

    return parts.join("/") + "/";
  }.property('Key'),

  grandParentKey: function() {
    var parts = this.get('keyParts').toArray();

    parts.pop();
    parts.pop();

    return parts.join("/") + "/";
  }.property('Key')
});
