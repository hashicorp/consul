// if we can't find the message, take the last part of the identifier and
// ucfirst it so it looks human
export default function missingMessage(key, locales) {
  const last = key
    .split('.')
    .pop()
    .replaceAll('-', ' ');
  return `${last.substr(0, 1).toUpperCase()}${last.substr(1)}`;
}
