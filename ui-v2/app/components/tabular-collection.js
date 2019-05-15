import CollectionComponent from 'ember-collection/components/ember-collection';
import needsRevalidate from 'ember-collection/utils/needs-revalidate';
import identity from 'ember-collection/utils/identity';
import Grid from 'ember-collection/layouts/grid';
import SlotsMixin from 'block-slots';
import WithResizing from 'consul-ui/mixins/with-resizing';
import style from 'ember-computed-style';

import { inject as service } from '@ember/service';
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
        const dom = get(this, 'dom');

        const $tr = dom.closest('tr', e.currentTarget);
        const $group = dom.sibling(e.currentTarget, 'ul');
        const groupRect = $group.getBoundingClientRect();
        const groupBottom = groupRect.top + $group.clientHeight;

        const $footer = dom.element('footer[role="contentinfo"]');
        const footerRect = $footer.getBoundingClientRect();
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
export default CollectionComponent.extend(SlotsMixin, WithResizing, {
  tagName: 'table',
  classNames: ['dom-recycling'],
  classNameBindings: ['hasActions'],
  attributeBindings: ['style'],
  width: 1150,
  rowHeight: 50,
  maxHeight: 500,
  style: style('getStyle'),
  checked: null,
  hasCaption: false,
  dom: service('dom'),
  init: function() {
    this._super(...arguments);
    this.change = change.bind(this);
    this.confirming = [];
    // TODO: The row height should auto calculate properly from the CSS
    this['cell-layout'] = new ZIndexedGrid(get(this, 'width'), get(this, 'rowHeight'));
  },
  getStyle: computed('rowHeight', '_items', 'maxRows', 'maxHeight', function() {
    const maxRows = get(this, 'rows');
    let height = get(this, 'maxHeight');
    if (maxRows) {
      let rows = Math.max(3, get(this._items || [], 'length'));
      rows = Math.min(maxRows, rows);
      height = get(this, 'rowHeight') * rows + 29;
    }
    return {
      height: height,
    };
  }),
  resize: function(e) {
    const $tbody = this.element;
    const dom = get(this, 'dom');
    const $appContent = dom.element('main > div');
    if ($appContent) {
      const border = 1;
      const rect = $tbody.getBoundingClientRect();
      const $footer = dom.element('footer[role="contentinfo"]');
      const space = rect.top + $footer.clientHeight + border;
      const height = e.detail.height - space;
      this.set('maxHeight', Math.max(0, height));
      // TODO: The row height should auto calculate properly from the CSS
      this['cell-layout'] = new ZIndexedGrid($appContent.clientWidth, get(this, 'rowHeight'));
      this.updateItems();
      this.updateScrollPosition();
    }
  },
  willRender: function() {
    this._super(...arguments);
    set(this, 'hasCaption', this._isRegistered('caption'));
    set(this, 'hasActions', this._isRegistered('actions'));
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
      return get(this, 'dom').clickFirstAnchor(e);
    },
  },
});
