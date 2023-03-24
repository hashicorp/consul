/* eslint no-console: "off" */
const log = function (results, measurement, tags) {
  console.log(measurement, results, tags);
};
export default function (len = 10000, report = log, performance = window.performance) {
  return function (cb, measurement, tags) {
    let actual;
    return new Array(len)
      .fill(true)
      .reduce(function (prev, item, i) {
        return prev.then(function (ms) {
          return new Promise(function (resolve) {
            const start = performance.now();
            cb().then(function (res) {
              actual = res;
              resolve(ms + (performance.now() - start));
            });
          });
        });
      }, Promise.resolve(0))
      .then(function (total) {
        report({ avg: total / len, total: total, count: len }, measurement, tags);
        return actual;
      });
  };
}
