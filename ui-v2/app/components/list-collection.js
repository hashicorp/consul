import { computed, get, set } from '@ember/object';
import Component from 'ember-collection/components/ember-collection';
import PercentageColumns from 'ember-collection/layouts/percentage-columns';
import style from 'ember-computed-style';
import WithResizing from 'consul-ui/mixins/with-resizing';
import qsaFactory from 'consul-ui/utils/dom/qsa-factory';
const $$ = qsaFactory();
export default Component.extend(WithResizing, {
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
    const $self = this.element;
    const $appContent = [...$$('main > div')][0];
    if ($appContent) {
      const rect = $self.getBoundingClientRect();
      const $footer = [...$$('footer[role="contentinfo"]')][0];
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
