export default function(encoded, raw, encode = encodeURIComponent) {
  return encoded.concat(raw.map(encode)).join('/');
}
