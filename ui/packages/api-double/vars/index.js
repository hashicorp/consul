module.exports = function(locationFactory, range, env) {
  env = typeof env === 'undefined' ? process.env : env;
  return function(vars) {
    return Object.assign(vars, {
      location: locationFactory(vars.href, vars.query),
      range: function(a, b) {
        if (typeof b !== 'undefined') {
          b = parseInt(b);
        }
        return range(parseInt(a), b);
      },
      http: {
        headers: vars.headers || {},
        body: vars.body || {},
        method: vars.method || 'GET',
        cookies: vars.cookies || {},
      },
      env: function(key, def) {
        key = key.toUpperCase();
        return (
          [
            this.http.cookies[key],
            env[key],
            this.http.headers[`X-${key.replace('_', '-')}`],
            def,
            null,
          ].reduce(
            function(prev, item) {
              if(typeof prev !== 'undefined') {
                return prev;
              }
              return item;
            }
          )
        );
      },
    });
  };
};

