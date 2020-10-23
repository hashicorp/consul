module.exports = function(resolve, find, render, YAML, mutate) {
  const log = console.log;
  mutate =
    mutate ||
    function(path, result, config) {
      return result;
    };
  const setTimeoutPromised = function(cb, ms) {
    return new Promise(function(resolve, reject) {
      setTimeout(function() {
        resolve(cb());
      }, ms);
    });
  };
  const makeConfig = function(config = {}, method = 'GET', params = {}) {
    const methodConfig = config[method] || {};
    Object.keys(methodConfig || {}).forEach(function(key) {
      config = Object.assign({}, methodConfig['*'] || {}, methodConfig[key]['*'] || {});
    });
    return config;
  };
  const getConfig = function(config) {
    return YAML.safeLoad(render(config))['*'];
  };
  return function(request, response, next) {
    let config = {};
    find(resolve(request.path))
      .then(function(obj) {
        var body;
        try {
          body = JSON.parse(request.body || {});
        } catch (e) {
          body = {};
        }
        const vars = {
          href: request.path,
          query: request.query,
          headers: request.headers,
          body: body,
          method: request.method,
          cookies: request.cookies,
        };
        if(request.cookies['API_DOUBLE_LOGS']) {
          log(`${request.method.toUpperCase()}: ${request.url}`);
        }
        config = getConfig(Object.assign({}, vars, { content: obj.config }));
        config = makeConfig(config, request.method, request.query);

        return render(Object.assign({}, vars, { content: obj.content }));
      })
      .then(function(result) {
        result = mutate(request, result, config);
        let status = 200;
        if (config.headers) {
          if(config.headers.response) {
            response.set(config.headers.response);
            if(config.headers.response['Status-Code']) {
              status = config.headers.response['Status-Code'];
            }
          }
        }
        switch (true) {
          case config.latency != null:
            return setTimeoutPromised(function() {
              return response.status(status).send(result);
            }, config.latency);
            break;
          default:
            return response.status(status).send(result);
        }
      })
      .catch(function(e) {
        console.error(e);
        next();
      });
  };
};
