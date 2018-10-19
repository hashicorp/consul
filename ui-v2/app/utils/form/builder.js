import { get, set, computed } from '@ember/object';
import Changeset from 'ember-changeset';
import lookupValidator from 'ember-changeset-validations';

// Keep these here for now so forms are easy to make
// TODO: Probably move this to utils/form/parse-element-name
import parseElementName from 'consul-ui/utils/get-form-name-property';
const defaultChangeset = function(data, validators) {
  const changeset = new Changeset(data, lookupValidator(validators), validators);
  // TODO: Currently supporting ember-data nicely like this
  changeset.isSaving = computed('data.isSaving', function() {
    return get(this, 'data.isSaving');
  });
  return changeset;
};
/**
 * Form builder/Form factory (WIP)
 * Deals with handling (generally change) events and updating data in response to the change
 * in a typical data down event up manner
 * validations are included currently using ember-changeset-validations
 *
 * @param {string} name - The name of the form, generally this is the name of your model
 *   Generally (until view building is complete) you should name your form elements as `name="modelName[property]"`
 *   or pass this name through using you action and create an Event-like object instead
 *   You can also just not set a name and use `name="property"`, but if you want to use combinations
 *   if multiple forms at least form children should use names
 *
 * @param {object} config - Form configuration object. Just a plain object to configure the form should be a hash
 *   with property names relating to the form data. Each property is the configuration for that model/data property
 *   currently the only supported property of these configuration objects is `type` which currently allows you to
 *   set a property as 'array-like'
 */
export default function(changeset = defaultChangeset, getFormNameProperty = parseElementName) {
  return function(name = '', obj = {}) {
    let _data;
    const _name = name;
    const _children = {};
    let _validators = null;
    // TODO make this into a class to reuse prototype
    return {
      getName: function() {
        return _name;
      },
      setData: function(data) {
        // Array check temporarily for when we get an empty array from repo.status
        if (_validators && !Array.isArray(data)) {
          _data = changeset(data, _validators);
        } else {
          _data = data;
        }
        return this;
      },
      getData: function() {
        return _data;
      },
      add: function(child) {
        _children[child.getName()] = child;
        return this;
      },
      handleEvent: function(e) {
        const target = e.target;
        const parts = getFormNameProperty(target.name);
        // split the form element name from `name[prop]`
        const name = parts[0];
        const prop = parts[1];
        //
        let config = obj;
        // if the name (usually the name of the model) isn't this form, look at its children
        if (name !== _name) {
          if (this.has(name)) {
            // is its a child form then use the child form
            return this.form(name).handleEvent(e);
          }
          // should probably throw here, unless we support having a name
          // even if you are referring to this form
          config = config[name];
        }
        const data = this.getData();
        // ember-data/changeset dance
        const json = typeof data.toJSON === 'function' ? data.toJSON() : get(data, 'data').toJSON();
        // if the form doesn't include a property then throw so it can be
        // caught outside, therefore the user can deal with things that aren't in the data
        if (!Object.keys(json).includes(prop)) {
          const error = new Error(`${prop} property doesn't exist`);
          error.target = target;
          throw error;
        }
        // deal with the change of property
        let currentValue = get(data, prop);
        // if the value is an array-like or config says its an array
        if (
          Array.isArray(currentValue) ||
          (typeof config[prop] !== 'undefined' &&
            typeof config[prop].type === 'string' &&
            config[prop].type.toLowerCase() === 'array')
        ) {
          // array specific set
          if (currentValue == null) {
            currentValue = [];
          }
          const method = target.checked ? 'pushObject' : 'removeObject';
          currentValue[method](target.value);
          set(data, prop, currentValue);
        } else {
          // deal with booleans
          // but only booleans that aren't checkboxes/radios with values
          if (
            typeof target.checked !== 'undefined' &&
            (target.value.toLowerCase() === 'on' || target.value.toLowerCase() === 'off')
          ) {
            set(data, prop, target.checked);
          } else {
            // text and non-boolean checkboxes/radios
            set(data, prop, target.value);
          }
        }
        // validate everything
        return this.validate();
      },
      reset: function() {
        const data = this.getData();
        if (typeof data.rollbackAttributes === 'function') {
          this.getData().rollbackAttributes();
        }
        return this;
      },
      setValidators: function(validators) {
        _validators = validators;
        return this;
      },
      validate: function() {
        const data = this.getData();
        // just pass along to the Changeset for now
        if (typeof data.validate === 'function') {
          data.validate();
        }
        return this;
      },
      addError: function(name, message) {
        const data = this.getData();
        if (typeof data.addError === 'function') {
          data.addError(...arguments);
        }
      },
      form: function(name) {
        if (name == null) {
          return this;
        }
        return _children[name];
      },
      has: function(name) {
        return typeof _children[name] !== 'undefined';
      },
    };
  };
}
