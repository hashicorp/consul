import Component from '@glimmer/component';

export default class SearchProvider extends Component {
  get items() {
    const { items, search, searchProperties } = this.args;

    if (search.length > 0) {
      const regex = new RegExp(`${search}`, 'ig');

      return items.filter((item) => {
        const matchesInSearchProperties = searchProperties.reduce((acc, searchProperty) => {
          const match = item[searchProperty].match(regex);
          if (match) {
            return [...acc, match];
          } else {
            return acc;
          }
        }, []);
        return matchesInSearchProperties.length > 0;
      });
    } else {
      return items;
    }
  }

  get data() {
    const { items } = this;
    return {
      items,
    };
  }
}
