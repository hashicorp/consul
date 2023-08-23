import Fuse from 'fuse.js';

export default class FuzzySearch {
  constructor(items, options) {
    this.fuse = new Fuse(items, {
      includeMatches: true,

      shouldSort: false, // use our own sorting for the moment
      threshold: 0.4,
      keys: Object.keys(options.finders) || [],
      getFn(item, key) {
        return (options.finders[key[0]](item) || []).toString();
      },
    });
  }

  search(s) {
    return this.fuse.search(s).map(item => item.item);
  }
}
