import Component from '@ember/component';
import { set } from '@ember/object';
import { inject as service } from '@ember/service';

export default Component.extend({
  dom: service('dom'),
  classNames: ['phrase-editor'],
  item: '',
  onchange: function(e) {},
  search: function(e) {
    // TODO: Temporarily continue supporting `searchable`
    let searchable = this.searchable;
    if (searchable) {
      if (!Array.isArray(searchable)) {
        searchable = [searchable];
      }
      searchable.forEach(item => {
        item.search(this.value);
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
          if (e.target.value == '' && this.value.length > 0) {
            this.actions.remove.bind(this)(this.value.length - 1);
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
      this.value.removeAt(index, 1);
      this.search({ target: this });
      this.input.focus();
    },
    add: function(e) {
      const item = this.item.trim();
      if (item !== '') {
        set(this, 'item', '');
        const currentItems = this.value || [];
        const items = new Set(currentItems).add(item);
        if (items.size > currentItems.length) {
          set(this, 'value', [...items]);
          this.search({ target: this });
        }
      }
    },
  },
});
