module.exports = function(resolve, dir) {
  return function(url = '') {
    return resolve(dir + url) + (url.substr(url.length - 1) === '/' ? '/index' : '');
  };
};
