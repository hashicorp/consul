/**
 * Turns a separated path or 'key name' in this case to
 * an array. If the key name is simply the separator (for example '/')
 * then the array should contain a single empty string value
 *
 * @param {string} key - The separated path/key
 * @param {string} [separator=/] - The separator
 * @returns {string[]}
 */
export default function(key, separator = '/') {
  return (key === separator ? '' : key).split(separator);
}
