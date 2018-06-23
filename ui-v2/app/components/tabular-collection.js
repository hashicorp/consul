import Component from 'ember-collection/components/ember-collection';
import needsRevalidate from 'ember-collection/utils/needs-revalidate';
import identity from 'ember-collection/utils/identity';
import Grid from 'ember-collection/layouts/grid';
import SlotsMixin from 'ember-block-slots';
import style from 'ember-computed-style';
import qsaFactory from 'consul-ui/utils/qsa-factory';

import { computed, get, set } from '@ember/object';
/**
 * Heavily extended `ember-collection` component
 * This adds support for z-index calculations to enable
 * Popup menus to pop over either rows above or below
 * the popup.
 * Additionally adds calculations for figuring out what the height
 * of the tabular component should be depending on the other elements
 * in the page.
 * Currently everything is here together for clarity, but to be split up
 * in the future
 */

// ember doesn't like you using `$` hence `$$`
const $$ = qsaFactory();
// basic pseudo CustomEvent interface
// TODO: use actual custom events once I've reminded
// myself re: support/polyfills
const createSizeEvent = function(detail) {
  return {
    detail: { width: window.innerWidth, height: window.innerHeight },
  };
};
// need to copy Cell in wholesale as there is no way to import it
// there is no change made to `Cell` here, its only here as its
// private in `ember-collection`
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
// this is an amount of rows in the table NOT items
// unlikely to have 10000 DOM rows ever :)
const maxZIndex = 10000;
// Adds z-index styling to the default Grid
class ZIndexedGrid extends Grid {
  formatItemStyle(index, w, h, checked) {
    let style = super.formatItemStyle(index, w, h);
    // count backwards from maxZIndex
    let zIndex = maxZIndex - index;
    // apart from the row that contains an opened dropdown menu
    // this one should be highest z-index, so use max plus 1
    if (checked == index) {
      zIndex = maxZIndex + 1;
    }
    style += 'z-index: ' + zIndex;
    return style;
  }
}
// basic DOM closest utility to cope with no support
// TODO: instead of degrading gracefully
// add a while polyfill for closest
const closest = function(sel, el) {
  try {
    return el.closest(sel);
  } catch (e) {
    return;
  }
};
const sibling = function(el, name) {
  let sibling = el;
  while ((sibling = sibling.nextSibling)) {
    if (sibling.nodeType === 1) {
      if (sibling.nodeName.toLowerCase() === name) {
        return sibling;
      }
    }
  }
};
/**
 * The tabular-collection can contain 'actions' the UI for which
 * uses dropdown 'action groups', so a group of different actions.
 * State makes use of native HTML state using radiogroups
 * to ensure that only a single dropdown can be open at one time.
 * Therefore we listen to change events to do anything extra when
 * a dropdown is opened (the change function is bound to the instance of
 * the `tabular-component` on init, hoisted here for visibility)
 *
 * The extra functionality we have here is to detect whether the opened
 * dropdown menu would be cut off or not if it 'dropped down'.
 * If it would be cut off we use CSS to 'drop it up' instead.
 * We also set this row to have the max z-index here, and mark this
 * row as the 'checked row' for when a scroll/grid re-calculation is
 * performed
 */
const change = function(e) {
  if (e instanceof MouseEvent) {
    return;
  }
  // TODO: Why am I getting a jQuery event here?!
  if (e instanceof Event) {
    const value = e.currentTarget.value;
    if (value != get(this, 'checked')) {
      set(this, 'checked', value);
      // 'actions_close' would mean that all menus have been closed
      // therefore we don't need to calculate
      if (e.currentTarget.getAttribute('id') !== 'actions_close') {
        const $tr = closest('tr', e.currentTarget);
        const $group = sibling(e.currentTarget, 'ul');
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
    // TODO: Consider moving all DOM lookups here
    // this seems to be the earliest place I can get them
    window.addEventListener('resize', this.handler);
    this.didAppear();
  },
  willDestroyElement: function() {
    window.removeEventListener('resize', this.handler);
  },
  didAppear: function() {
    this.handler();
  },
  resize: function(e) {
    const $tbody = [...$$('tbody', this.element)][0];
    const $appContent = [...$$('main > div')][0];
    if ($appContent) {
      const rect = $tbody.getBoundingClientRect();
      const $footer = [...$$('footer[role="contentinfo"]')][0];
      const space = rect.top + $footer.clientHeight;
      const height = new Number(e.detail.height - space);
      this.set('height', Math.max(0, height));
      // TODO: The row height should auto calculate properly from the CSS
      this['cell-layout'] = new ZIndexedGrid($appContent.clientWidth, 50);
      this.updateItems();
      this.updateScrollPosition();
    }
  },
  // `ember-collection` bug workaround
  // https://github.com/emberjs/ember-collection/issues/138
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
  // unfortunately the nicest way I could think to do this is to copy this in wholesale
  // to add an extra argument for `formatItemStyle` in 3 places
  // tradeoff between changing as little code as possible in the original code
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
      // click on row functionality
      // so if you click the actual row but not a link
      // find the first link and fire that instead
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
