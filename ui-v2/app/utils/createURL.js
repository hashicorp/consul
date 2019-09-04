export default function(encode) {
  return function(strs, ...values) {
    return strs
      .map(function(item, i) {
        let val = typeof values[i] === 'undefined' ? '' : values[i];
        switch (true) {
          case typeof val === 'string':
            val = encode(val);
            break;
          case Array.isArray(val):
            val = val
              .map(function(item) {
                return `${encode(item)}`;
              }, '')
              .join('/');
            break;
          case typeof val === 'object':
            val = Object.keys(val)
              .reduce(function(prev, key) {
                if (val[key] === null) {
                  return prev.concat(`${encode(key)}`);
                } else if (typeof val[key] !== 'undefined') {
                  return prev.concat(`${encode(key)}=${encode(val[key])}`);
                }
                return prev;
              }, [])
              .join('&');
            break;
        }
        return `${item}${val}`;
      })
      .join('')
      .trim();
  };
}
