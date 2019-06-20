import Component from '@ember/component';
import { get, set } from '@ember/object';
import { inject as service } from '@ember/service';

export default Component.extend({
  dom: service('dom'),
  classNames: ['phrase-editor'],
  item: '',
  didInsertElement: function() {
    this._super(...arguments);
    // TODO: use {{ref}}
    this.input = get(this, 'dom').element('input', this.element);
  },
  onchange: function(e) {},
  search: function(e) {
    // TODO: Temporarily continue supporting `searchable`
    let searchable = get(this, 'searchable');
    if (searchable) {
      if (!Array.isArray(searchable)) {
        searchable = [searchable];
      }
      searchable.forEach(item => {
        item.search(get(this, 'value'));
      });
    }
    this.onchange(e);
  },
  oninput: function(e) {},
  onkeydown: function(e) {},
  actions: {
    keydown: function(e) {
      switch (e.keyCode) {
        case 8: // backspace
          if (e.target.value == '' && get(this, 'value').length > 0) {
            this.actions.remove.bind(this)(get(this, 'value').length - 1);
          }
          break;
        case 27: // escape
          set(this, 'value', []);
          this.search({ target: this });
          break;
      }
      this.onkeydown({ target: this });
    },
    input: function(e) {
      set(this, 'item', e.target.value);
      this.oninput({ target: this });
    },
    remove: function(index, e) {
      get(this, 'value').removeAt(index, 1);
      this.search({ target: this });
      this.input.focus();
    },
    add: function(e) {
      const item = get(this, 'item').trim();
      if (item !== '') {
        set(this, 'item', '');
        const currentItems = get(this, 'value') || [];
        const items = new Set(currentItems).add(item);
        if (items.size > currentItems.length) {
          set(this, 'value', [...items]);
          this.search({ target: this });
        }
      }
    },
  },
});
