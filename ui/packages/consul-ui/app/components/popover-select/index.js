import Component from '@ember/component';
import { inject as service } from '@ember/service';
import Slotted from 'block-slots';

export default Component.extend(Slotted, {
  tagName: '',
  dom: service('dom'),
  multiple: false,
  required: false,
  onchange: function() {},
  addOption: function(option) {
    if (typeof this._options === 'undefined') {
      this._options = new Set();
    }
    this._options.add(option);
  },
  removeOption: function(option) {
    this._options.delete(option);
  },
  actions: {
    click: function(option, e) {
      // required={{true}} ?
      if (!this.multiple) {
        if (option.selected && this.required) {
          return e;
        }
        [...this._options]
          .filter(item => item !== option)
          .forEach(item => {
            item.selected = false;
          });
      } else {
        if (option.selected && this.required) {
          const other = [...this._options].find(item => item !== option && item.selected);
          if (!other) {
            return e;
          }
        }
      }
      option.selected = !option.selected;
      this.onchange(
        this.dom.setEventTargetProperties(e, {
          selected: target => option.args.value,
          selectedItems: target => {
            return [...this._options]
              .filter(item => item.selected)
              .map(item => item.args.value)
              .join(',');
          },
        })
      );
      return e;
    },
  },
});
