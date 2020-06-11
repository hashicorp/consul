export default function(encode) {
  // TODO: This is incredibly similar to create-query-params
  // we should be able to merge these
  const jsonToQueryParams = function(val) {
    return Object.entries(val)
      .reduce(function(prev, [key, value]) {
        if (value === null) {
          return prev.concat(`${encode(key)}`);
        } else if (typeof value !== 'undefined') {
          return prev.concat(`${encode(key)}=${encode(value)}`);
        }
        return prev;
      }, [])
      .join('&');
  };
  return function(strs, ...values) {
    // TODO: Potentially url should check if any of the params
    // passed to it are undefined (null is fine). We could then get rid of the
    // multitude of checks we do throughout the adapters
    // right now createURL converts undefined to '' so we need to check thats not needed
    // anywhere
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
            val = jsonToQueryParams(val);
            break;
        }
        return `${item}${val}`;
      })
      .join('')
      .trim();
  };
}
