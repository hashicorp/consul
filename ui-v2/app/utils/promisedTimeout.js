export default function(P = Promise, timeout = setTimeout) {
  // var interval;
  return function(milliseconds, cb) {
    // clearInterval(interval);
    // const cb = typeof _cb !== 'function' ? (i) => { clearInterval(interval);interval = i; } : _cb;
    return new P((resolve, reject) => {
      cb(
        timeout(function() {
          resolve(milliseconds);
        }, milliseconds)
      );
    });
  };
}
