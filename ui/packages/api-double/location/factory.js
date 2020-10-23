module.exports = function() {
  return function(path, query) {
    var split = path.split('/');
    return {
      search: query,
      pathname: {
        toString: function() {
          return path;
        },
        valueOf: function() {
          return path;
        },
        slice: function(a, b) {
          return split.slice(...arguments).join('/');
        },
        isDir: function() {
          return path.lastIndexOf('/') === path.length - 1;
        },
        // array access without complications
        get: function(i) {
          // skip the empty zero index
          return split[i + 1];
        },
      },
    };
  };
};
