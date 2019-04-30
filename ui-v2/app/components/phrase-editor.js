import Component from '@ember/component';
import { get, set } from '@ember/object';

export default Component.extend({
  classNames: ['phrase-editor'],
  item: '',
  remove: function(index, e) {
    this.items.removeAt(index, 1);
    this.onchange(e);
  },
  add: function(e) {
    const value = get(this, 'item').trim();
    if (value !== '') {
      set(this, 'item', '');
      const currentItems = get(this, 'items') || [];
      const items = new Set(currentItems).add(value);
      if (items.size > currentItems.length) {
        set(this, 'items', [...items]);
        this.onchange(e);
      }
    }
  },
  onkeydown: function(e) {
    switch (e.keyCode) {
      case 8:
        if (e.target.value == '' && this.items.length > 0) {
          this.remove(this.items.length - 1);
        }
        break;
    }
  },
  oninput: function(e) {
    set(this, 'item', e.target.value);
  },
  onchange: function(e) {
    let searchable = get(this, 'searchable');
    if (!Array.isArray(searchable)) {
      searchable = [searchable];
    }
    searchable.forEach(item => {
      item.search(get(this, 'items'));
    });
  },
});
