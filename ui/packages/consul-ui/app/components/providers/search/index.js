import Component from '@glimmer/component';

export default class SearchProvider extends Component {
  get items() {
    const { items, search, searchProperties } = this.args;

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
  }

  get data() {
    const { items } = this;
    return {
      items,
    };
  }
}
