import Component from '@ember/component';
import { inject as service } from '@ember/service';
import { set } from '@ember/object';

export default Component.extend({
  dom: service('dom'),
  // TODO(octane): Remove when we can move to glimmer components
  // so we aren't using ember-test-selectors
  // supportsDataTestProperties: true,
  // the above doesn't seem to do anything so still need to find a way
  // to pass through data-test-properties
  tagName: '',
  // TODO: reserved for the moment but we don't need it yet
  onblur: null,
  checked: false,
  onchange: function() {},
  init: function() {
    this._super(...arguments);
    this.guid = this.dom.guid(this);
    this._listeners = this.dom.listeners();
  },
  didInsertElement: function() {
    this._super(...arguments);
    // TODO(octane): move to ref
    set(this, 'input', this.dom.element(`#toggle-button-${this.guid}`));
    set(this, 'label', this.input.nextElementSibling);
  },
  willDestroyElement: function() {
    this._super(...arguments);
    this._listeners.remove();
  },
  actions: {
    click: function(e) {
      e.preventDefault();
      this.input.checked = !this.input.checked;
      // manually dispatched mouse events have a detail = 0
      // real mouse events have the number of click counts
      if (e.detail !== 0) {
        e.target.blur();
      }
      this.actions.change.apply(this, [e]);
    },
    change: function(e) {
      if (this.input.checked) {
        // default onblur event
        this._listeners.remove();
        this._listeners.add(this.dom.document(), 'click', e => {
          if (this.dom.isOutside(this.label, e.target)) {
            if (this.dom.isOutside(this.label.nextElementSibling, e.target)) {
              if (this.input.checked) {
                this.input.checked = false;
                // TODO: This should be an event
                this.onchange({ target: this.input });
              }
              this._listeners.remove();
            }
          }
        });
      }
      // TODO: This should be an event
      this.onchange({ target: this.input });
    },
  },
});
