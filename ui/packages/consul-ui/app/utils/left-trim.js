export default function leftTrim(str = '', search = '') {
  return str.indexOf(search) === 0 ? str.substr(search.length) : str;
}
