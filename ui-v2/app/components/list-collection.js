import { inject as service } from '@ember/service';
import { computed, get, set } from '@ember/object';
import Component from 'ember-collection/components/ember-collection';
import PercentageColumns from 'ember-collection/layouts/percentage-columns';
import style from 'ember-computed-style';
import WithResizing from 'consul-ui/mixins/with-resizing';

export default Component.extend(WithResizing, {
  dom: service('dom'),
  tagName: 'div',
  attributeBindings: ['style'],
  height: 500,
  cellHeight: 113,
  style: style('getStyle'),
  classNames: ['list-collection'],
  init: function() {
    this._super(...arguments);
    this.columns = [25, 25, 25, 25];
  },
  didReceiveAttrs: function() {
    this._super(...arguments);
    this._cellLayout = this['cell-layout'] = new PercentageColumns(
      get(this, 'items.length'),
      get(this, 'columns'),
      get(this, 'cellHeight')
    );
  },
  getStyle: computed('height', function() {
    return {
      height: get(this, 'height'),
    };
  }),
  resize: function(e) {
    // TODO: This top part is very similar to resize in tabular-collection
    // see if it make sense to DRY out
    const dom = get(this, 'dom');
    const $appContent = dom.element('main > div');
    if ($appContent) {
      const rect = this.element.getBoundingClientRect();
      const $footer = dom.element('footer[role="contentinfo"]');
      const space = rect.top + $footer.clientHeight;
      const height = e.detail.height - space;
      this.set('height', Math.max(0, height));
      this.updateItems();
      this.updateScrollPosition();
    }
    const width = e.detail.width;
    const len = get(this, 'columns.length');
    switch (true) {
      case width > 1013:
        if (len != 4) {
          set(this, 'columns', [25, 25, 25, 25]);
        }
        break;
      case width > 744:
        if (len != 3) {
          set(this, 'columns', [33, 33, 34]);
        }
        break;
      case width > 487:
        if (len != 2) {
          set(this, 'columns', [50, 50]);
        }
        break;
      case width < 488:
        if (len != 1) {
          set(this, 'columns', [100]);
        }
    }
    if (len !== get(this, 'columns.length')) {
      this._cellLayout = this['cell-layout'] = new PercentageColumns(
        get(this, 'items.length'),
        get(this, 'columns'),
        get(this, 'cellHeight')
      );
    }
  },
});
