module.exports = function(read, root) {
  const find = function(p, wildcard) {
    if (
      p
        .split('/')
        .slice(0, -1)
        .join('/') == root
    ) {
      return Promise.reject(root);
    }
    return read(p)
      .then(function(content) {
        const temp = p.split('/');
        temp.pop();
        temp.push('.config');
        const config = temp.join('/');
        return read(config)
          .then(function(config) {
            return {
              config: config.toString(), // temp match all for now
              content: content.toString(),
            };
          })
          .catch(function(e) {
            return {
              config: {},
              content: content.toString(),
            };
          });
      })
      .catch(function() {
        const temp = p.split('/');
        const last = temp.slice(-1)[0];
        if (last == wildcard) {
          temp.pop();
        }
        if (temp.length > 0) {
          return find(
            temp
              .slice(0, -1)
              .concat([wildcard])
              .join('/'),
            wildcard
          );
        }
      });
  };
  return function(path, wildcard = '_') {
    return find(path, wildcard).catch(function(root) {
      return Promise.reject(path);
    });
  };
};
