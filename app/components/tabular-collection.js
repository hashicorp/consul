import Component from 'ember-collection/components/ember-collection';
import needsRevalidate from 'ember-collection/utils/needs-revalidate';

export default Component.extend({
  tagName: '',
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
