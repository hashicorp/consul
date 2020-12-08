import Component from '@ember/component';
import { inject as service } from '@ember/service';
import Slotted from 'block-slots';

export default Component.extend(Slotted, {
  tagName: '',
  dom: service('dom'),
  multiple: false,
  subtractive: false,
  onchange: function() {},
  addOption: function(option) {
    if (typeof this._options === 'undefined') {
      this._options = new Set();
    }
    if (this.subtractive) {
      if (!option.selected) {
        this._options.add(option.value);
      }
    } else {
      if (option.selected) {
        this._options.add(option.value);
      }
    }
  },
  removeOption: function(option) {
    this._options.delete(option.value);
  },
  actions: {
    click: function(e, value) {
      let options = [value];
      if (this.multiple) {
        if (this._options.has(value)) {
          this._options.delete(value);
        } else {
          this._options.add(value);
        }
        options = this._options;
      }
      this.onchange(
        this.dom.setEventTargetProperties(e, {
          selected: target => value,
          selectedItems: target => {
            return [...options].join(',');
          },
        })
      );
    },
    change: function(option, e) {
      this.onchange(this.dom.setEventTargetProperty(e, 'selected', selected => option));
    },
  },
});
