// const METHOD_PARSING = 0;
const PATH_PARSING = 1;
const QUERY_PARSING = 2;
const HEADER_PARSING = 3;
const BODY_PARSING = 4;
export default function (encode, queryParams) {
  return function (strs, ...values) {
    // TODO: Potentially url should check if any of the params
    // passed to it are undefined (null is fine). We could then get rid of the
    // multitude of checks we do throughout the adapters
    // right now create-url converts undefined to '' so we need to check thats not needed
    // anywhere
    let state = PATH_PARSING;
    return strs
      .map(function (item, i, arr) {
        if (i === 0) {
          item = item.trimStart();
        }
        // if(item.indexOf(' ') !== -1 && state === METHOD_PARSING) {
        //   state = PATH_PARSING;
        // }
        if (item.indexOf('?') !== -1 && state === PATH_PARSING) {
          state = QUERY_PARSING;
        }
        if (item.indexOf('\n\n') !== -1) {
          state = BODY_PARSING;
        }
        if (item.indexOf('\n') !== -1 && state !== BODY_PARSING) {
          state = HEADER_PARSING;
        }
        let val = typeof values[i] !== 'undefined' ? values[i] : '';
        switch (state) {
          case PATH_PARSING:
            switch (true) {
              // encode strings
              case typeof val === 'string':
                val = encode(val);
                break;
              // split encode and join arrays by `/`
              case Array.isArray(val):
                val = val
                  .map(function (item) {
                    return `${encode(item)}`;
                  }, '')
                  .join('/');
                break;
            }
            break;
          case QUERY_PARSING:
            switch (true) {
              case typeof val === 'string':
                val = encode(val);
                break;
              // objects offload to queryParams for encoding
              case typeof val === 'object':
                val = queryParams(val);
                break;
            }
            break;
          case BODY_PARSING:
            // ignore body until we parse it here
            return item.split('\n\n')[0];
          // case METHOD_PARSING:
          case HEADER_PARSING:
          // passthrough/ignore method and headers until we parse them here
        }
        return `${item}${val}`;
      })
      .join('')
      .trim();
  };
}
