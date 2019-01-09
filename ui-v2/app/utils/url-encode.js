/**
 * url-encode takes arrays of strings or recursive arrays of arrays and strings
 * and returns an array with the strings encoded, if you want to pass
 * something but not encode the slashes, then you should split the string
 * by slash first and pass the array
 */
export default function(encode = encodeURIComponent) {
  return function(arr) {
    return arr.reduce(function reducer(prev, item) {
      if (Array.isArray(item)) {
        return prev.concat([item.reduce(reducer, []).join('/')]);
      } else {
        prev.push(encode(item));
        return prev;
      }
    }, []);
  };
}
