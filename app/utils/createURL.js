export default function(encoded, raw, query = {}, encode = encodeURIComponent) {
  return [
    encoded.concat(raw.map(encode)).join('/'),
    Object.keys(query)
      .map(function(key, i, arr) {
        if (query[key] != null) {
          return `${key}=${encode(query[key])}`;
        }
        return key;
      })
      .join('&'),
  ]
    .filter(item => item !== '')
    .join('?');
}
