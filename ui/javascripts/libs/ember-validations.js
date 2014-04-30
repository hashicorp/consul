// ==========================================================================
// Project:   Ember Validations
// Copyright: Copyright 2013 DockYard, LLC. and contributors.
// License:   Licensed under MIT license (see license.js)
// ==========================================================================


 // Version: 1.0.0.beta.2

(function() {
Ember.Validations = Ember.Namespace.create({
  VERSION: '1.0.0.beta.2'
});

})();



(function() {
Ember.Validations.messages = {
  render: function(attribute, context) {
    if (Ember.I18n) {
      return Ember.I18n.t('errors.' + attribute, context);
    } else {
      var regex = new RegExp("{{(.*?)}}"),
          attributeName = "";
      if (regex.test(this.defaults[attribute])) {
        attributeName = regex.exec(this.defaults[attribute])[1];
      }
      return this.defaults[attribute].replace(regex, context[attributeName]);
    }
  },
  defaults: {
    inclusion: "is not included in the list",
    exclusion: "is reserved",
    invalid: "is invalid",
    confirmation: "doesn't match {{attribute}}",
    accepted: "must be accepted",
    empty: "can't be empty",
    blank: "can't be blank",
    present: "must be blank",
    tooLong: "is too long (maximum is {{count}} characters)",
    tooShort: "is too short (minimum is {{count}} characters)",
    wrongLength: "is the wrong length (should be {{count}} characters)",
    notANumber: "is not a number",
    notAnInteger: "must be an integer",
    greaterThan: "must be greater than {{count}}",
    greaterThanOrEqualTo: "must be greater than or equal to {{count}}",
    equalTo: "must be equal to {{count}}",
    lessThan: "must be less than {{count}}",
    lessThanOrEqualTo: "must be less than or equal to {{count}}",
    otherThan: "must be other than {{count}}",
    odd: "must be odd",
    even: "must be even",
    url: "is not a valid URL"
  }
};

})();



(function() {
Ember.Validations.Errors = Ember.Object.extend({
  unknownProperty: function(property) {
    this.set(property, Ember.makeArray());
    return this.get(property);
  }
});

})();



(function() {
var setValidityMixin = Ember.Mixin.create({
  isValid: function() {
    return this.get('validators').compact().filterBy('isValid', false).get('length') === 0;
  }.property('validators.@each.isValid'),
  isInvalid: Ember.computed.not('isValid')
});

var pushValidatableObject = function(model, property) {
  var content = model.get(property);

  model.removeObserver(property, pushValidatableObject);
  if (Ember.isArray(content)) {
    model.validators.pushObject(ArrayValidatorProxy.create({model: model, property: property, contentBinding: 'model.' + property}));
  } else {
    model.validators.pushObject(content);
  }
};

var findValidator = function(validator) {
  var klass = validator.classify();
  return Ember.Validations.validators.local[klass] || Ember.Validations.validators.remote[klass];
};

var ArrayValidatorProxy = Ember.ArrayProxy.extend(setValidityMixin, {
  validate: function() {
    return this._validate();
  },
  _validate: function() {
    var promises = this.get('content').invoke('_validate').without(undefined);
    return Ember.RSVP.all(promises);
  }.on('init'),
  validators: Ember.computed.alias('content')
});

Ember.Validations.Mixin = Ember.Mixin.create(setValidityMixin, {
  init: function() {
    this._super();
    this.errors = Ember.Validations.Errors.create();
    this._dependentValidationKeys = {};
    this.validators = Ember.makeArray();
    if (this.get('validations') === undefined) {
      this.validations = {};
    }
    this.buildValidators();
    this.validators.forEach(function(validator) {
      validator.addObserver('errors.[]', this, function(sender, key, value, context, rev) {
        var errors = Ember.makeArray();
        this.validators.forEach(function(validator) {
          if (validator.property === sender.property) {
            errors = errors.concat(validator.errors);
          }
        }, this);
        this.set('errors.' + sender.property, errors);
      });
    }, this);
  },
  buildValidators: function() {
    var property, validator;

    for (property in this.validations) {
      if (this.validations[property].constructor === Object) {
        this.buildRuleValidator(property);
      } else {
        this.buildObjectValidator(property);
      }
    }
  },
  buildRuleValidator: function(property) {
    var validator;
    for (validator in this.validations[property]) {
      if (this.validations[property].hasOwnProperty(validator)) {
        this.validators.pushObject(findValidator(validator).create({model: this, property: property, options: this.validations[property][validator]}));
      }
    }
  },
  buildObjectValidator: function(property) {
    if (Ember.isNone(this.get(property))) {
      this.addObserver(property, this, pushValidatableObject);
    } else {
      pushValidatableObject(this, property);
    }
  },
  validate: function() {
    var self = this;
    return this._validate().then(function(vals) {
      var errors = self.get('errors');
      if (vals.contains(false)) {
        return Ember.RSVP.reject(errors);
      }
      return errors;
    });
  },
  _validate: function() {
    var promises = this.validators.invoke('_validate').without(undefined);
    return Ember.RSVP.all(promises);
  }.on('init')
});

})();



(function() {
Ember.Validations.patterns = Ember.Namespace.create({
  numericality: /^(-|\+)?(?:\d+|\d{1,3}(?:,\d{3})+)(?:\.\d*)?$/,
  blank: /^\s*$/
});

})();



(function() {
Ember.Validations.validators        = Ember.Namespace.create();
Ember.Validations.validators.local  = Ember.Namespace.create();
Ember.Validations.validators.remote = Ember.Namespace.create();

})();



(function() {
Ember.Validations.validators.Base = Ember.Object.extend({
  init: function() {
    this.set('errors', Ember.makeArray());
    this._dependentValidationKeys = Ember.makeArray();
    this.conditionals = {
      'if': this.get('options.if'),
      unless: this.get('options.unless')
    };
    this.model.addObserver(this.property, this, this._validate);
  },
  addObserversForDependentValidationKeys: function() {
    this._dependentValidationKeys.forEach(function(key) {
      this.model.addObserver(key, this, this._validate);
    }, this);
  }.on('init'),
  pushDependentValidationKeyToModel: function() {
    var model = this.get('model');
    if (model._dependentValidationKeys[this.property] === undefined) {
      model._dependentValidationKeys[this.property] = Ember.makeArray();
    }
    model._dependentValidationKeys[this.property].addObjects(this._dependentValidationKeys);
  }.on('init'),
  call: function () {
    throw 'Not implemented!';
  },
  unknownProperty: function(key) {
    var model = this.get('model');
    if (model) {
      return model.get(key);
    }
  },
  isValid: Ember.computed.empty('errors.[]'),
  validate: function() {
    var self = this;
    return this._validate().then(function(success) {
      // Convert validation failures to rejects.
      var errors = self.get('model.errors');
      if (success) {
        return errors;
      } else {
        return Ember.RSVP.reject(errors);
      }
    });
  },
  _validate: function() {
    this.errors.clear();
    if (this.canValidate()) {
      this.call();
    }
    if (this.get('isValid')) {
      return Ember.RSVP.resolve(true);
    } else {
      return Ember.RSVP.resolve(false);
    }
  }.on('init'),
  canValidate: function() {
    if (typeof(this.conditionals) === 'object') {
      if (this.conditionals['if']) {
        if (typeof(this.conditionals['if']) === 'function') {
          return this.conditionals['if'](this.model, this.property);
        } else if (typeof(this.conditionals['if']) === 'string') {
          if (typeof(this.model[this.conditionals['if']]) === 'function') {
            return this.model[this.conditionals['if']]();
          } else {
            return this.model.get(this.conditionals['if']);
          }
        }
      } else if (this.conditionals.unless) {
        if (typeof(this.conditionals.unless) === 'function') {
          return !this.conditionals.unless(this.model, this.property);
        } else if (typeof(this.conditionals.unless) === 'string') {
          if (typeof(this.model[this.conditionals.unless]) === 'function') {
            return !this.model[this.conditionals.unless]();
          } else {
            return !this.model.get(this.conditionals.unless);
          }
        }
      } else {
        return true;
      }
    } else {
      return true;
    }
  }
});

})();



(function() {
Ember.Validations.validators.local.Absence = Ember.Validations.validators.Base.extend({
  init: function() {
    this._super();
    /*jshint expr:true*/
    if (this.options === true) {
      this.set('options', {});
    }

    if (this.options.message === undefined) {
      this.set('options.message', Ember.Validations.messages.render('present', this.options));
    }
  },
  call: function() {
    if (!Ember.isEmpty(this.model.get(this.property))) {
      this.errors.pushObject(this.options.message);
    }
  }
});

})();



(function() {
Ember.Validations.validators.local.Acceptance = Ember.Validations.validators.Base.extend({
  init: function() {
    this._super();
    /*jshint expr:true*/
    if (this.options === true) {
      this.set('options', {});
    }

    if (this.options.message === undefined) {
      this.set('options.message', Ember.Validations.messages.render('accepted', this.options));
    }
  },
  call: function() {
    if (this.options.accept) {
      if (this.model.get(this.property) !== this.options.accept) {
        this.errors.pushObject(this.options.message);
      }
    } else if (this.model.get(this.property) !== '1' && this.model.get(this.property) !== 1 && this.model.get(this.property) !== true) {
      this.errors.pushObject(this.options.message);
    }
  }
});

})();



(function() {
Ember.Validations.validators.local.Confirmation = Ember.Validations.validators.Base.extend({
  init: function() {
    this.originalProperty = this.property;
    this.property = this.property + 'Confirmation';
    this._super();
    this._dependentValidationKeys.pushObject(this.originalProperty);
    /*jshint expr:true*/
    if (this.options === true) {
      this.set('options', { attribute: this.originalProperty });
      this.set('options', { message: Ember.Validations.messages.render('confirmation', this.options) });
    }
  },
  call: function() {
    if (this.model.get(this.originalProperty) !== this.model.get(this.property)) {
      this.errors.pushObject(this.options.message);
    }
  }
});

})();



(function() {
Ember.Validations.validators.local.Exclusion = Ember.Validations.validators.Base.extend({
  init: function() {
    this._super();
    if (this.options.constructor === Array) {
      this.set('options', { 'in': this.options });
    }

    if (this.options.message === undefined) {
      this.set('options.message', Ember.Validations.messages.render('exclusion', this.options));
    }
  },
  call: function() {
    /*jshint expr:true*/
    var message, lower, upper;

    if (Ember.isEmpty(this.model.get(this.property))) {
      if (this.options.allowBlank === undefined) {
        this.errors.pushObject(this.options.message);
      }
    } else if (this.options['in']) {
      if (Ember.$.inArray(this.model.get(this.property), this.options['in']) !== -1) {
        this.errors.pushObject(this.options.message);
      }
    } else if (this.options.range) {
      lower = this.options.range[0];
      upper = this.options.range[1];

      if (this.model.get(this.property) >= lower && this.model.get(this.property) <= upper) {
        this.errors.pushObject(this.options.message);
      }
    }
  }
});

})();



(function() {
Ember.Validations.validators.local.Format = Ember.Validations.validators.Base.extend({
  init: function() {
    this._super();
    if (this.options.constructor === RegExp) {
      this.set('options', { 'with': this.options });
    }

    if (this.options.message === undefined) {
      this.set('options.message',  Ember.Validations.messages.render('invalid', this.options));
    }
   },
   call: function() {
    if (Ember.isEmpty(this.model.get(this.property))) {
      if (this.options.allowBlank === undefined) {
        this.errors.pushObject(this.options.message);
      }
    } else if (this.options['with'] && !this.options['with'].test(this.model.get(this.property))) {
      this.errors.pushObject(this.options.message);
    } else if (this.options.without && this.options.without.test(this.model.get(this.property))) {
      this.errors.pushObject(this.options.message);
    }
  }
});

})();



(function() {
Ember.Validations.validators.local.Inclusion = Ember.Validations.validators.Base.extend({
  init: function() {
    this._super();
    if (this.options.constructor === Array) {
      this.set('options', { 'in': this.options });
    }

    if (this.options.message === undefined) {
      this.set('options.message', Ember.Validations.messages.render('inclusion', this.options));
    }
  },
  call: function() {
    var message, lower, upper;
    if (Ember.isEmpty(this.model.get(this.property))) {
      if (this.options.allowBlank === undefined) {
        this.errors.pushObject(this.options.message);
      }
    } else if (this.options['in']) {
      if (Ember.$.inArray(this.model.get(this.property), this.options['in']) === -1) {
        this.errors.pushObject(this.options.message);
      }
    } else if (this.options.range) {
      lower = this.options.range[0];
      upper = this.options.range[1];

      if (this.model.get(this.property) < lower || this.model.get(this.property) > upper) {
        this.errors.pushObject(this.options.message);
      }
    }
  }
});

})();



(function() {
Ember.Validations.validators.local.Length = Ember.Validations.validators.Base.extend({
  init: function() {
    var index, key;
    this._super();
    /*jshint expr:true*/
    if (typeof(this.options) === 'number') {
      this.set('options', { 'is': this.options });
    }

    if (this.options.messages === undefined) {
      this.set('options.messages', {});
    }

    for (index = 0; index < this.messageKeys().length; index++) {
      key = this.messageKeys()[index];
      if (this.options[key] !== undefined && this.options[key].constructor === String) {
        this.model.addObserver(this.options[key], this, this._validate);
      }
    }

    this.options.tokenizer = this.options.tokenizer || function(value) { return value.split(''); };
    // if (typeof(this.options.tokenizer) === 'function') {
      // debugger;
      // // this.tokenizedLength = new Function('value', 'return '
    // } else {
      // this.tokenizedLength = new Function('value', 'return (value || "").' + (this.options.tokenizer || 'split("")') + '.length');
    // }
  },
  CHECKS: {
    'is'      : '==',
    'minimum' : '>=',
    'maximum' : '<='
  },
  MESSAGES: {
    'is'      : 'wrongLength',
    'minimum' : 'tooShort',
    'maximum' : 'tooLong'
  },
  getValue: function(key) {
    if (this.options[key].constructor === String) {
      return this.model.get(this.options[key]) || 0;
    } else {
      return this.options[key];
    }
  },
  messageKeys: function() {
    return Ember.keys(this.MESSAGES);
  },
  checkKeys: function() {
    return Ember.keys(this.CHECKS);
  },
  renderMessageFor: function(key) {
    var options = {count: this.getValue(key)}, _key;
    for (_key in this.options) {
      options[_key] = this.options[_key];
    }

    return this.options.messages[this.MESSAGES[key]] || Ember.Validations.messages.render(this.MESSAGES[key], options);
  },
  renderBlankMessage: function() {
    if (this.options.is) {
      return this.renderMessageFor('is');
    } else if (this.options.minimum) {
      return this.renderMessageFor('minimum');
    }
  },
  call: function() {
    var check, fn, message, operator, key;

    if (Ember.isEmpty(this.model.get(this.property))) {
      if (this.options.allowBlank === undefined && (this.options.is || this.options.minimum)) {
        this.errors.pushObject(this.renderBlankMessage());
      }
    } else {
      for (key in this.CHECKS) {
        operator = this.CHECKS[key];
        if (!this.options[key]) {
          continue;
        }

        fn = new Function('return ' + this.options.tokenizer(this.model.get(this.property)).length + ' ' + operator + ' ' + this.getValue(key));
        if (!fn()) {
          this.errors.pushObject(this.renderMessageFor(key));
        }
      }
    }
  }
});

})();



(function() {
Ember.Validations.validators.local.Numericality = Ember.Validations.validators.Base.extend({
  init: function() {
    /*jshint expr:true*/
    var index, keys, key;
    this._super();

    if (this.options === true) {
      this.options = {};
    } else if (this.options.constructor === String) {
      key = this.options;
      this.options = {};
      this.options[key] = true;
    }

    if (this.options.messages === undefined || this.options.messages.numericality === undefined) {
      this.options.messages = this.options.messages || {};
      this.options.messages = { numericality: Ember.Validations.messages.render('notANumber', this.options) };
    }

    if (this.options.onlyInteger !== undefined && this.options.messages.onlyInteger === undefined) {
      this.options.messages.onlyInteger = Ember.Validations.messages.render('notAnInteger', this.options);
    }

    keys = Ember.keys(this.CHECKS).concat(['odd', 'even']);
    for(index = 0; index < keys.length; index++) {
      key = keys[index];

      if (isNaN(this.options[key])) {
        this.model.addObserver(this.options[key], this, this._validate);
      }

      if (this.options[key] !== undefined && this.options.messages[key] === undefined) {
        if (Ember.$.inArray(key, Ember.keys(this.CHECKS)) !== -1) {
          this.options.count = this.options[key];
        }
        this.options.messages[key] = Ember.Validations.messages.render(key, this.options);
        if (this.options.count !== undefined) {
          delete this.options.count;
        }
      }
    }
  },
  CHECKS: {
    equalTo              :'===',
    greaterThan          : '>',
    greaterThanOrEqualTo : '>=',
    lessThan             : '<',
    lessThanOrEqualTo    : '<='
  },
  call: function() {
    var check, checkValue, fn, form, operator, val;

    if (Ember.isEmpty(this.model.get(this.property))) {
      if (this.options.allowBlank === undefined) {
        this.errors.pushObject(this.options.messages.numericality);
      }
    } else if (!Ember.Validations.patterns.numericality.test(this.model.get(this.property))) {
      this.errors.pushObject(this.options.messages.numericality);
    } else if (this.options.onlyInteger === true && !(/^[+\-]?\d+$/.test(this.model.get(this.property)))) {
      this.errors.pushObject(this.options.messages.onlyInteger);
    } else if (this.options.odd  && parseInt(this.model.get(this.property), 10) % 2 === 0) {
      this.errors.pushObject(this.options.messages.odd);
    } else if (this.options.even && parseInt(this.model.get(this.property), 10) % 2 !== 0) {
      this.errors.pushObject(this.options.messages.even);
    } else {
      for (check in this.CHECKS) {
        operator = this.CHECKS[check];

        if (this.options[check] === undefined) {
          continue;
        }

        if (!isNaN(parseFloat(this.options[check])) && isFinite(this.options[check])) {
          checkValue = this.options[check];
        } else if (this.model.get(this.options[check]) !== undefined) {
          checkValue = this.model.get(this.options[check]);
        }

        fn = new Function('return ' + this.model.get(this.property) + ' ' + operator + ' ' + checkValue);

        if (!fn()) {
          this.errors.pushObject(this.options.messages[check]);
        }
      }
    }
  }
});

})();



(function() {
Ember.Validations.validators.local.Presence = Ember.Validations.validators.Base.extend({
  init: function() {
    this._super();
    /*jshint expr:true*/
    if (this.options === true) {
      this.options = {};
    }

    if (this.options.message === undefined) {
      this.options.message = Ember.Validations.messages.render('blank', this.options);
    }
  },
  call: function() {
    if (Ember.isEmpty(this.model.get(this.property))) {
      this.errors.pushObject(this.options.message);
    }
  }
});

})();



(function() {
Ember.Validations.validators.local.Url = Ember.Validations.validators.Base.extend({
  regexp: null,
  regexp_ip: null,

  init: function() {
    this._super();

    if (this.get('options.message') === undefined) {
      this.set('options.message', Ember.Validations.messages.render('url', this.options));
    }

    if (this.get('options.protocols') === undefined) {
      this.set('options.protocols', ['http', 'https']);
    }

    // Regular Expression Parts
    var dec_octet = '(25[0-5]|2[0-4][0-9]|[0-1][0-9][0-9]|[1-9][0-9]|[0-9])'; // 0-255
    var ipaddress = '(' + dec_octet + '(\\.' + dec_octet + '){3})';
    var hostname = '([a-zA-Z0-9\\-]+\\.)+([a-zA-Z]{2,})';
    var encoded = '%[0-9a-fA-F]{2}';
    var characters = 'a-zA-Z0-9$\\-_.+!*\'(),;:@&=';
    var segment = '([' + characters + ']|' + encoded + ')*';

    // Build Regular Expression
    var regex_str = '^';

    if (this.get('options.domainOnly') === true) {
      regex_str += hostname;
    } else {
      regex_str += '(' + this.get('options.protocols').join('|') + '):\\/\\/'; // Protocol

      // Username and password
      if (this.get('options.allowUserPass') === true) {
        regex_str += '(([a-zA-Z0-9$\\-_.+!*\'(),;:&=]|' + encoded + ')+@)?'; // Username & passwords
      }

      // IP Addresses?
      if (this.get('options.allowIp') === true) {
        regex_str += '(' + hostname + '|' + ipaddress + ')'; // Hostname OR IP
      } else {
        regex_str += '(' + hostname + ')'; // Hostname only
      }

      // Ports
      if (this.get('options.allowPort') === true) {
        regex_str += '(:[0-9]+)?'; // Port
      }

      regex_str += '(\\/';
      regex_str += '(' + segment + '(\\/' + segment + ')*)?'; // Path
      regex_str += '(\\?' + '([' + characters + '/?]|' + encoded + ')*)?'; // Query
      regex_str += '(\\#' + '([' + characters + '/?]|' + encoded + ')*)?'; // Anchor
      regex_str += ')?';
    }

    regex_str += '$';

    // RegExp
    this.regexp = new RegExp(regex_str);
    this.regexp_ip = new RegExp(ipaddress);
  },
  call: function() {
    var url = this.model.get(this.property);

    if (Ember.isEmpty(url)) {
      if (this.get('options.allowBlank') !== true) {
        this.errors.pushObject(this.get('options.message'));
      }
    } else {
      if (this.get('options.allowIp') !== true) {
        if (this.regexp_ip.test(url)) {
          this.errors.pushObject(this.get('options.message'));
          return;
        }
      }

      if (!this.regexp.test(url)) {
        this.errors.pushObject(this.get('options.message'));
      }
    }
  }
});


})();



(function() {

})();



(function() {

})();

