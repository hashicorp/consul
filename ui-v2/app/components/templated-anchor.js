import Component from '@ember/component';
import { get, set, computed } from '@ember/object';

const createWeak = function(wm = new WeakMap()) {
  return {
    get: function(ref, prop) {
      let map = wm.get(ref);
      if (map) {
        return map[prop];
      }
    },
    set: function(ref, prop, value) {
      let map = wm.get(ref);
      if (typeof map === 'undefined') {
        map = {};
        wm.set(ref, map);
      }
      map[prop] = value;
      return map[prop];
    },
  };
};
const weak = createWeak();
// Covers alpha-capitalized dot separated API keys such as
// `{{Name}}`, `{{Service.Name}}` etc. but not `{{}}`
const templateRe = /{{([A-Za-z.0-9_-]+)}}/g;
export default Component.extend({
  tagName: 'a',
  attributeBindings: ['href', 'rel', 'target'],
  rel: computed({
    get: function(prop) {
      return weak.get(this, prop);
    },
    set: function(prop, value) {
      switch (value) {
        case 'external':
          value = `${value} noopener noreferrer`;
          set(this, 'target', '_blank');
          break;
      }
      return weak.set(this, prop, value);
    },
  }),
  vars: computed({
    get: function(prop) {
      return weak.get(this, prop);
    },
    set: function(prop, value) {
      weak.set(this, prop, value);
      set(this, 'href', weak.get(this, 'template'));
    },
  }),
  href: computed({
    get: function(prop) {
      return weak.get(this, prop);
    },
    set: function(prop, value) {
      weak.set(this, 'template', value);
      const vars = weak.get(this, 'vars');
      if (typeof vars !== 'undefined' && typeof value !== 'undefined') {
        value = value.replace(templateRe, function(match, group) {
          try {
            return encodeURIComponent(get(vars, group) || '');
          } catch (e) {
            return '';
          }
        });
        return weak.set(this, prop, value);
      }
      return '';
    },
  }),
});
