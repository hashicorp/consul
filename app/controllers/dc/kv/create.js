import Controller from '@ember/controller';
import { get, set } from '@ember/object';
const btoa = window.btoa;
export default Controller.extend({
  json: false,
  actions: {
    change: function(e) {
        const target = e.target || {name: 'value', value: e};
        switch(target.name) {
          case 'basename':
            set(this, 'item.Key', `${get(this, 'parentKey')}${target.value}`)
            break;
          case 'json':
            set(this, 'json', !get(this, 'json'))
            break;
          case 'value':
            set(this, 'item.Value', btoa(target.value))
            break;
        }
    },
  }
});
