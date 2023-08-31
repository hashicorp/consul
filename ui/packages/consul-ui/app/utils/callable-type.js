export default function (obj) {
  if (typeof obj !== 'function') {
    return function () {
      return obj;
    };
  } else {
    return obj;
  }
}
