import { helper } from '@ember/component/helper';

/**
 * Conditionally maps classInfos (classes) to a string ready for typical DOM
 * usage (i.e. space delimited)
 *
 * @typedef {([string, boolean] | [string])} classInfo
 * @param {(classInfo | string)[]} entries - An array of 'entry-like' arrays of `classInfo`s to map
 */
const classMap = (entries) => {
  const str = entries
    .filter(Boolean)
    .filter((entry) => (typeof entry === 'string' ? true : entry[entry.length - 1]))
    .map((entry) => (typeof entry === 'string' ? entry : entry[0]))
    .join(' ');
  return str.length > 0 ? str : undefined;
};
export default helper(classMap);
