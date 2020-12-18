import PredicateSearch from './predicate';

export default class ExactSearch extends PredicateSearch {
  predicate(s) {
    s = s.toLowerCase();
    return (item = '') =>
      item
        .toString()
        .toLowerCase()
        .indexOf(s) !== -1;
  }
}
