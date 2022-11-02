export default class PredicateSearch {
  constructor(items, options) {
    this.items = items;
    this.options = options;
  }

  search(s) {
    const predicate = this.predicate(s);
    // Test the value of each key for each object against the regex
    // All that match are returned.
    return this.items.filter((item) => {
      return Object.entries(this.options.finders).some(([key, finder]) => {
        const val = finder(item);
        if (Array.isArray(val)) {
          return val.some(predicate);
        } else {
          return predicate(val);
        }
      });
    });
  }
}
