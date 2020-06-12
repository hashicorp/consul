export default function(encode, queryParams) {
  return function(strs, ...values) {
    // TODO: Potentially url should check if any of the params
    // passed to it are undefined (null is fine). We could then get rid of the
    // multitude of checks we do throughout the adapters
    // right now create-url converts undefined to '' so we need to check thats not needed
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
            val = queryParams(val);
            break;
        }
        return `${item}${val}`;
      })
      .join('')
      .trim();
  };
}
