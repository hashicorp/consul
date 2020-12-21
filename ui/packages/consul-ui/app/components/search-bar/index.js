import Component from '@glimmer/component';
import { action } from '@ember/object';

const diff = (a, b) => {
  return a.filter(item => !b.includes(item));
};
export default class SearchBar extends Component {
  get isFiltered() {
    const searchproperty = this.args.filter.searchproperty;
    return (
      diff(searchproperty.default, searchproperty.value).length > 0 ||
      Object.entries(this.args.filter).some(([key, value]) => {
        return key !== 'searchproperty' && typeof value.value !== 'undefined';
      })
    );
  }
  get filters() {
    const filters = Object.entries(this.args.filter)
      .filter(([key, value]) => {
        if (key === 'searchproperty') {
          return diff(value.default, value.value).length > 0;
        }
        return (value.value || []).length > 0;
      })
      .reduce((prev, [key, value]) => {
        return prev.concat(
          value.value.map(item => {
            const obj = {
              key: key,
              value: item,
            };
            if (key !== 'searchproperty') {
              obj.selected = diff(value.value, [item]);
            } else {
              obj.selected = value.value.length === 1 ? value.default : diff([item], value.value);
            }
            return obj;
          })
        );
      }, []);
    return filters;
  }
  @action
  removeFilters() {
    Object.values(this.args.filter).forEach((value, i) => {
      // put in a little queue to ensure query params are unset properly
      // ideally this would be done outside of the component
      setTimeout(() => value.change(value.default || []), 1 * i);
    });
  }
}
