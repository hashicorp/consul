export default function rightTrim(str = '', search = '') {
  const pos = str.length - search.length;
  return str.lastIndexOf(search) === pos ? str.substr(0, pos) : str;
}
