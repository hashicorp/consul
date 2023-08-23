export default function(name) {
  if (name.indexOf('[') !== -1) {
    return name.match(/(.*)\[(.*)\]/).slice(1);
  }
  return ['', name];
}
