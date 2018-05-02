export default function(P = Promise, timeout = setTimeout) {
  return function(milliseconds, cb) {
    return new P((resolve, reject) => {
      cb(
        timeout(function() {
          resolve(milliseconds);
        }, milliseconds)
      );
    });
  };
}
