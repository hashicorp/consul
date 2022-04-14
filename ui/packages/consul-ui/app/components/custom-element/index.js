import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';
import { assert } from '@ember/debug';

const ATTRIBUTE_CHANGE = 'custom-element.attributeChange';
const elements = new Map();
const proxies = new WeakMap();

const typeCast = (attributeInfo, value) => {
  let type = attributeInfo.type;
  const d = attributeInfo.default;
  value = value == null ? attributeInfo.default : value;
  if(type.indexOf('|') !== -1) {
    assert(`"${value} is not of type '${type}'"`, type.split('|').map(item => item.replaceAll('"', '').trim()).includes(value));
    type = 'string';
  }
  switch(type) {
    case '<length>':
    case '<percentage>':
    case '<dimension>':
    case 'number': {
      const num = parseFloat(value);
      if(isNaN(num)) {
        return typeof d === 'undefined' ? 0 : d;
      } else {
        return num;
      }
    }
    case '<integer>':
      return parseInt(value);
    case '<string>':
    case 'string':
      return (value || '').toString();
  }
}

const attributeChangingElement = (name, Cls = HTMLElement, attributes = {}, cssprops = {}) => {
  const attrs = Object.keys(attributes);

  const customClass = class extends Cls {
    static get observedAttributes() {
      return attrs;
    }

    attributeChangedCallback(name, oldValue, newValue) {
      const prev = typeCast(attributes[name], oldValue);
      const value = typeCast(attributes[name], newValue);

      const cssProp = cssprops[`--${name}`];
      if(typeof cssProp !== 'undefined' && cssProp.track === `[${name}]`) {
        this.style.setProperty(
          `--${name}`,
          value
        );
      }

      if(typeof super.attributeChangedCallback === 'function') {
        super.attributeChangedCallback(name, prev, value);
      }

      this.dispatchEvent(
        new CustomEvent(
          ATTRIBUTE_CHANGE,
          {
            detail: {
              name: name,
              previousValue: prev,
              value: value
            }
          }
        )
      );

    }
  }
  customElements.define(name, customClass);
  return () => {};
}

const infoFromArray = (arr, keys) => {
  return (arr || []).reduce((prev, info) => {
    let key;
    const obj = {};
    keys.forEach((item, i) => {
      if(item === '_') {
        key = i;
        return;
      }
      obj[item] = info[i]
    });
    prev[info[key]] = obj;
    return prev;
  }, {});
}
const debounceRAF = (cb, prev) => {
  if(typeof prev !== 'undefined') {
    cancelAnimationFrame(prev);
  }
  return requestAnimationFrame(cb);
}
const createElementProxy = ($element, component) => {
  return new Proxy($element, {
    get: (target, prop, receiver) => {
      switch(prop) {
        case 'attrs':
          return component.attributes;
        default:
          if(typeof target[prop] === 'function') {
            // need to ensure we use a MultiWeakMap here
            // if(this.methods.has(prop)) {
            //   return this.methods.get(prop);
            // }
            const method = target[prop].bind(target);
            // this.methods.set(prop, method);
            return method;
          }

      }
    }
  });
}

export default class CustomElementComponent extends Component {

  @tracked $element;
  @tracked _attributes = {};

  __attributes;
  _attchange;


  constructor(owner, args) {
    super(...arguments);
    if(!elements.has(args.element)) {
      const cb = attributeChangingElement(
        args.element,
        args.class,
        infoFromArray(args.attrs, ['_', 'type', 'default', 'description']),
        infoFromArray(args.cssprops, ['_', 'type', 'track', 'description'])
      )
      elements.set(args.element, cb);
    }
  }

  get attributes() {
    return this._attributes;
  }

  get element() {
    if(this.$element) {
      if(proxies.has(this.$element)) {
        return proxies.get(this.$element);
      }
      const proxy = createElementProxy(this.$element, this);
      proxies.set(this.$element, proxy);
      return proxy;
    }
    return undefined;
  }

  @action
  setHost(attachShadow, $element) {
    attachShadow($element);
    this.$element = $element;
    this.$element.addEventListener(ATTRIBUTE_CHANGE, this.attributeChange);

    (this.args.attrs || []).forEach(entry => {
      const value = $element.getAttribute(entry[0]);
      $element.attributeChangedCallback(entry[0], value, value)
    });
  }

  @action
  disconnect() {
    this.$element.removeEventListener(ATTRIBUTE_CHANGE, this.attributeChange);
  }

  @action
  attributeChange(e) {
    e.stopImmediatePropagation();
    // currently if one single attribute changes
    // they all change
    this.__attributes = {
      ...this.__attributes,
      [e.detail.name]: e.detail.value
    };
    this._attchange = debounceRAF(() => {
      // tell glimmer we changed the attrs
      this._attributes = this.__attributes;
    }, this._attchange);
  }
}
