import Component from 'ember-collection/components/ember-collection';
import needsRevalidate from 'ember-collection/utils/needs-revalidate';
import identity from 'ember-collection/utils/identity';
import Grid from 'ember-collection/layouts/grid';
import SlotsMixin from 'ember-block-slots';
import style from 'ember-computed-style';

import { computed, get, set } from '@ember/object';

const $$ = function(sel, context = document) {
  return context.querySelectorAll(sel);
};
const createSizeEvent = function(detail) {
  return {
    detail: { width: window.innerWidth, height: window.innerHeight },
  };
};
// need to copy this in wholesale as there is no way to import it
// TODO: separate both Cell and ZIndexedGrid out
class Cell {
  constructor(key, item, index, style) {
    this.key = key;
    this.hidden = false;
    this.item = item;
    this.index = index;
    this.style = style;
  }
}
const maxZIndex = 10000;
class ZIndexedGrid extends Grid {
  formatItemStyle(index, w, h, checked) {
    let style = super.formatItemStyle(index, w, h);
    let zIndex = maxZIndex - index;
    if (checked == index) {
      zIndex = maxZIndex + 1;
    }
    style += 'z-index: ' + zIndex;
    return style;
  }
}
// TODO instead of degrading gracefully
// add a while polyfill for closest
const closest = function(sel, el) {
  try {
    return el.closest(sel);
  } catch (e) {
    return;
  }
};
const change = function(e) {
  if (e instanceof MouseEvent) {
    return;
  }
  // TODO: Why am I getting a jQuery event here?!
  if (e instanceof Event) {
    const value = e.currentTarget.value;
    if (value != get(this, 'checked')) {
      set(this, 'checked', value);
      if (e.currentTarget.getAttribute('id') !== 'actions_close') {
        const $tr = closest('tr', e.currentTarget);
        const $group = [...$('~ ul', e.currentTarget)][0];
        const $footer = [...$$('footer[role="contentinfo"]')][0];
        const groupRect = $group.getBoundingClientRect();
        const footerRect = $footer.getBoundingClientRect();
        const groupBottom = groupRect.top + $group.clientHeight;
        const footerTop = footerRect.top;
        if (groupBottom > footerTop) {
          $group.classList.add('above');
        } else {
          $group.classList.remove('above');
        }
        $tr.style.zIndex = maxZIndex + 1;
      }
    } else {
      set(this, 'checked', null);
    }
  } else if (e.detail && e.detail.index) {
    if (e.detail.confirming) {
      this.confirming.push(e.detail.index);
    } else {
      const pos = this.confirming.indexOf(e.detail.index);
      if (pos !== -1) {
        this.confirming.splice(pos, 1);
      }
    }
  }
};
export default Component.extend(SlotsMixin, {
  tagName: 'table',
  attributeBindings: ['style'],
  width: 1150,
  height: 500,
  style: style('getStyle'),
  checked: null,
  init: function() {
    this._super(...arguments);
    this.change = change.bind(this);
    this.confirming = [];
    // TODO: The row height should auto calculate properly from the CSS
    this['cell-layout'] = new ZIndexedGrid(get(this, 'width'), 50);
    this.handler = () => {
      this.resize(createSizeEvent());
    };
  },
  getStyle: computed('height', function() {
    return {
      height: get(this, 'height'),
    };
  }),
  willRender: function() {
    this._super(...arguments);
    this.set('hasActions', this._isRegistered('actions'));
  },
  didInsertElement: function() {
    this._super(...arguments);
    window.addEventListener('resize', this.handler);
    this.handler();
  },
  willDestroyElement: function() {
    window.removeEventListener('resize', this.handler);
  },
  resize: function(e) {
    const $tbody = this.$('tbody').get(0);
    if ($tbody) {
      const rect = $tbody.getBoundingClientRect();
      const $footer = [...$$('footer[role="contentinfo"]')][0];
      const space = rect.top + $footer.clientHeight;
      const height = new Number(e.detail.height - space);
      this.set('height', Math.max(0, height));
      // TODO: The row height should auto calculate properly from the CSS
      this['cell-layout'] = new ZIndexedGrid($tbody.clientWidth, 50);
      this.updateItems();
      this.updateScrollPosition();
    }
  },
  _needsRevalidate: function() {
    if (this.isDestroyed || this.isDestroying) {
      return;
    }
    if (this._isGlimmer2()) {
      this.rerender();
    } else {
      needsRevalidate(this);
    }
  },
  // need to overwrite this completely so I can pass through the checked index
  // for `formatItemStyle` in 3 places
  updateCells: function() {
    if (!this._items) {
      return;
    }
    const numItems = get(this._items, 'length');
    if (this._cellLayout.length !== numItems) {
      this._cellLayout.length = numItems;
    }

    var priorMap = this._cellMap;
    var cellMap = Object.create(null);

    var index = this._cellLayout.indexAt(
      this._scrollLeft,
      this._scrollTop,
      this._clientWidth,
      this._clientHeight
    );
    var count = this._cellLayout.count(
      this._scrollLeft,
      this._scrollTop,
      this._clientWidth,
      this._clientHeight
    );
    var items = this._items;
    var bufferBefore = Math.min(index, this._buffer);
    index -= bufferBefore;
    count += bufferBefore;
    count = Math.min(count + this._buffer, get(items, 'length') - index);
    var i, style, itemIndex, itemKey, cell;

    var newItems = [];

    for (i = 0; i < count; i++) {
      itemIndex = index + i;
      itemKey = identity(items.objectAt(itemIndex));
      if (priorMap) {
        cell = priorMap[itemKey];
      }
      if (cell) {
        // additional `checked` argument
        style = this._cellLayout.formatItemStyle(
          itemIndex,
          this._clientWidth,
          this._clientHeight,
          this.checked
        );
        set(cell, 'style', style);
        set(cell, 'hidden', false);
        set(cell, 'key', itemKey);
        cellMap[itemKey] = cell;
      } else {
        newItems.push(itemIndex);
      }
    }

    for (i = 0; i < this._cells.length; i++) {
      cell = this._cells[i];
      if (!cellMap[cell.key]) {
        if (newItems.length) {
          itemIndex = newItems.pop();
          let item = items.objectAt(itemIndex);
          itemKey = identity(item);
          // additional `checked` argument
          style = this._cellLayout.formatItemStyle(
            itemIndex,
            this._clientWidth,
            this._clientHeight,
            this.checked
          );
          set(cell, 'style', style);
          set(cell, 'key', itemKey);
          set(cell, 'index', itemIndex);
          set(cell, 'item', item);
          set(cell, 'hidden', false);
          cellMap[itemKey] = cell;
        } else {
          set(cell, 'hidden', true);
          set(cell, 'style', 'height: 0; display: none;');
        }
      }
    }

    for (i = 0; i < newItems.length; i++) {
      itemIndex = newItems[i];
      let item = items.objectAt(itemIndex);
      itemKey = identity(item);
      // additional `checked` argument
      style = this._cellLayout.formatItemStyle(
        itemIndex,
        this._clientWidth,
        this._clientHeight,
        this.checked
      );
      cell = new Cell(itemKey, item, itemIndex, style);
      cellMap[itemKey] = cell;
      this._cells.pushObject(cell);
    }
    this._cellMap = cellMap;
  },
  actions: {
    click: function(e) {
      const name = e.target.nodeName.toLowerCase();
      switch (name) {
        case 'input':
        case 'label':
        case 'a':
        case 'button':
          return;
      }
      const $a = closest('tr', e.target).querySelector('a');
      if ($a) {
        const click = new MouseEvent('click', {
          bubbles: true,
          cancelable: true,
          view: window,
        });
        $a.dispatchEvent(click);
      }
    },
  },
});
