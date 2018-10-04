import Service from '@ember/service';
// TODO: Probably move this to utils/form/parse-element-name
import getFormNameProperty from 'consul-ui/utils/get-form-name-property';
import { get, set } from '@ember/object';
const builder = function(name = '', obj = {}) {
  let _data;
  const _name = name;
  const _children = {};
  // TODO make this into a class to reuse prototype
  return {
    getName: function() {
      return _name;
    },
    setData: function(data) {
      _data = data;
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
      let config = obj;
      // if the name isn't this form, look at its children
      if (name !== _name) {
        if (typeof _children[name] !== 'undefined') {
          return _children[name].handleEvent(e);
        }
        // should probably throw here, unless we support having a name
        // even if you are referring to this form
        config = config[name];
      }
      const data = this.getData();
      // ember-data/changeset dance
      const json = typeof data.toJSON === 'function' ? data.toJSON() : get(data, 'data').toJSON();
      if (!Object.keys(json).includes(prop)) {
        const error = new Error(`${prop} property doesn't exist`);
        error.target = target;
        throw error;
      }
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
        // if checks don't have values its a boolean
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
      return this.validate();
    },
    validate: function() {
      const data = this.getData();
      if (typeof data.validate === 'function') {
        data.validate();
      }
      return this;
    },
    // TODO: Decide whether I can keep things simple enough
    // to keep the form and the builder merged or whether should
    // use something like this to get the form instance
    // for now just return this
    form: function() {
      return this;
    },
  };
};
export default Service.extend({
  build: function(obj, name) {
    return builder(...arguments);
  },
});
