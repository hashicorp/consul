/**
 * Creates a safe url encoded url
 *
 * @param {string[]} encoded - Pre-encoded parts for the url
 * @param {string[]} raw - Possibly unsafe (not encoded) parts for the url
 * @param {object} query - A 'query object' the values of which are possibly unsafe and will be passed through encode
 * @param {function} [encode=encodeURIComponent] - Injectable encode function, defaulting to the browser default encodeURIComponent
 *
 * @example
 * // returns 'a/nice-url/with%20some/non%20encoded?sortBy=the%20name&page=1'
 * createURL(['a/nice-url'], ['with some', 'non encoded'], {sortBy: "the name", page: 1})
 */
export default function(encoded, raw, query = {}, encode = encodeURIComponent) {
  return [
    encoded.concat(raw).join('/'),
    Object.keys(query)
      .map(function(key, i, arr) {
        if (query[key] != null) {
          return `${encode(key)}=${encode(query[key])}`;
        }
        return key;
      })
      .join('&'),
  ]
    .filter(item => item !== '')
    .join('?');
}
