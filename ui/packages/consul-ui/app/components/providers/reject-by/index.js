import Component from '@glimmer/component';
import { isArray } from '@ember/array';
import { get } from '@ember/object';
import { isEmpty, isEqual, isPresent } from '@ember/utils';

export default class RejectByProvider extends Component {
  get items() {
    const { items, path, value } = this.args;

    if (!isArray) {
      return [];
    } else if (isEmpty(path)) {
      return items;
    }

    let filterFn;
    if (isPresent(value)) {
      if (typeof value === 'function') {
        filterFn = (item) => !value(get(item, path));
      } else {
        filterFn = (item) => !isEqual(get(item, path), value);
      }
    } else {
      filterFn = (item) => !get(item, path);
    }

    return items.filter(filterFn);
  }

  get data() {
    const { items } = this;
    return {
      items,
    };
  }
}
