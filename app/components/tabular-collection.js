import Component from 'ember-collection/components/ember-collection';
import needsRevalidate from 'ember-collection/utils/needs-revalidate';
import Grid from 'ember-collection/layouts/grid';
import SlotsMixin from 'ember-block-slots';
import style from 'ember-computed-style';

import { computed, get } from '@ember/object';

const $$ = document.querySelectorAll.bind(document);
const createSizeEvent = function(detail) {
  return {
    detail: { width: window.innerWidth, height: window.innerHeight },
  };
};

export default Component.extend(SlotsMixin, {
  tagName: 'table',
  attributeBindings: ['style'],
  width: 1150,
  height: 500,
  style: style('getStyle'),
  init: function() {
    this._super(...arguments);
    this['cell-layout'] = new Grid(get(this, 'width'), 46);
    this.handler = () => {
      this.resize(createSizeEvent());
    };
  },
  getStyle: computed('height', function() {
    return {
      height: get(this, 'height'),
    };
  }),
  didInsertElement: function() {
    this._super(...arguments);
    window.addEventListener('resize', this.handler);
    this.handler();
  },
  willDestroyElement: function() {
    window.removeEventListener('resize', this.handler);
  },
  resize: function(e) {
    // const $header = [...$('#wrapper > header')][0];
    const $footer = [...$$('#wrapper > footer')][0];
    // const $thead = this.$('thead')[0];
    // TODO: cheat for the moment
    const $thead = [...$$('main > div')][0];
    if ($thead) {
      this.set('height', new Number(e.detail.height - ($footer.clientHeight + 335)));
      this['cell-layout'] = new Grid($thead.clientWidth, 46);
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
});
