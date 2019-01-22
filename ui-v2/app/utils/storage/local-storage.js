export default function(
  scheme = '',
  storage = window.localStorage,
  encode = JSON.stringify,
  decode = JSON.parse
) {
  const prefix = `${scheme}:`;
  return {
    getValue: function(path) {
      let value = storage.getItem(`${prefix}${path}`);
      if (typeof value !== 'string') {
        value = '""';
      }
      try {
        value = decode(value);
      } catch (e) {
        value = '';
      }
      return value;
    },
    setValue: function(path, value) {
      if (value === null) {
        return this.removeValue(path);
      }
      try {
        value = encode(value);
      } catch (e) {
        value = '""';
      }
      return storage.setItem(`${prefix}${path}`, value);
    },
    removeValue: function(path) {
      return storage.removeItem(`${prefix}${path}`);
    },
    all: function() {
      return Object.keys(storage).reduce((prev, item, i, arr) => {
        if (item.indexOf(`${prefix}`) === 0) {
          const key = item.substr(prefix.length);
          prev[key] = this.getValue(key);
        }
        return prev;
      }, {});
    },
  };
}
