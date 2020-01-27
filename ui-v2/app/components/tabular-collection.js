import { inject as service } from '@ember/service';
import { computed, get, set } from '@ember/object';
import CollectionComponent from 'ember-collection/components/ember-collection';
import needsRevalidate from 'ember-collection/utils/needs-revalidate';
import Grid from 'ember-collection/layouts/grid';
import style from 'ember-computed-style';
import SlotsMixin from 'block-slots';
import WithResizing from 'consul-ui/mixins/with-resizing';

const formatItemStyle = Grid.prototype.formatItemStyle;

export default CollectionComponent.extend(SlotsMixin, WithResizing, {
  tagName: 'table',
  classNames: ['dom-recycling'],
  classNameBindings: ['hasActions'],
  attributeBindings: ['style'],
  dom: service('dom'),
  style: style('getStyle'),
  width: 1150,
  rowHeight: 50,
  maxHeight: 500,
  checked: null,
  hasCaption: false,
  init: function() {
    this._super(...arguments);
    // TODO: The row height should auto calculate properly from the CSS
    const o = this;
    this['cell-layout'] = new Grid(get(this, 'width'), get(this, 'rowHeight'));
    this['cell-layout'].formatItemStyle = function(itemIndex) {
      let style = formatItemStyle.apply(this, arguments);
      if (o.checked === itemIndex) {
        style = `${style};z-index: 1`;
      }
      return style;
    };
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
    const $appContent = this.dom.element('main > div');
    if ($appContent) {
      const border = 1;
      const rect = $tbody.getBoundingClientRect();
      const $footer = this.dom.element('footer[role="contentinfo"]');
      const space = rect.top + $footer.clientHeight + border;
      const height = e.detail.height - space;
      this.set('maxHeight', Math.max(0, height));
      // TODO: The row height should auto calculate properly from the CSS
      this['cell-layout'] = new Grid($appContent.clientWidth, get(this, 'rowHeight'));
      const o = this;
      this['cell-layout'].formatItemStyle = function(itemIndex) {
        let style = formatItemStyle.apply(this, arguments);
        if (o.checked === itemIndex) {
          style = `${style};z-index: 1`;
        }
        return style;
      };
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
  actions: {
    click: function(e) {
      return this.dom.clickFirstAnchor(e);
    },
    change: function(index, e = {}) {
      if (typeof index !== 'string') {
        return;
      }
      if (this.$tr) {
        this.$tr.style.zIndex = null;
      }
      if (e.target.checked && index != get(this, 'checked')) {
        set(this, 'checked', parseInt(index));
        const target = e.target;
        const $tr = this.dom.closest('tr', target);
        const $group = this.dom.sibling(target, 'div');
        const groupRect = $group.getBoundingClientRect();
        const groupBottom = groupRect.top + $group.clientHeight;

        const $footer = this.dom.element('footer[role="contentinfo"]');
        const footerRect = $footer.getBoundingClientRect();
        const footerTop = footerRect.top;

        if (groupBottom > footerTop) {
          $group.classList.add('above');
        } else {
          $group.classList.remove('above');
        }
        $tr.style.zIndex = 1;
        this.$tr = $tr;
      } else {
        set(this, 'checked', null);
      }
    },
  },
});
