export default function(scenario, assert, api) {
  const lastRequest = function(method) {
    return api.server.history
      .slice(0)
      .reverse()
      .find(function(item) {
        return item.method === method;
      });
  };
  scenario
    .then('a $method request is made to "$url" with the body from yaml\n$yaml', function(
      method,
      url,
      data
    ) {
      const request = api.server.history[api.server.history.length - 2];
      assert.equal(
        request.method,
        method,
        `Expected the request method to be ${method}, was ${request.method}`
      );
      assert.equal(request.url, url, `Expected the request url to be ${url}, was ${request.url}`);
      const body = JSON.parse(request.requestBody);
      Object.keys(data).forEach(function(key, i, arr) {
        assert.deepEqual(
          body[key],
          data[key],
          `Expected the payload to contain ${key} equaling ${data[key]}, ${key} was ${body[key]}`
        );
      });
    })
    // TODO: This one can replace the above one, it covers more use cases
    // also DRY it out a bit
    .then('a $method request is made to "$url" from yaml\n$yaml', function(method, url, yaml) {
      const request = api.server.history[api.server.history.length - 2];
      assert.equal(
        request.method,
        method,
        `Expected the request method to be ${method}, was ${request.method}`
      );
      assert.equal(request.url, url, `Expected the request url to be ${url}, was ${request.url}`);
      let data = yaml.body || {};
      const body = JSON.parse(request.requestBody);
      Object.keys(data).forEach(function(key, i, arr) {
        assert.equal(
          body[key],
          data[key],
          `Expected the payload to contain ${key} to equal ${body[key]}, ${key} was ${data[key]}`
        );
      });
      data = yaml.headers || {};
      const headers = request.requestHeaders;
      Object.keys(data).forEach(function(key, i, arr) {
        assert.equal(
          headers[key],
          data[key],
          `Expected the payload to contain ${key} to equal ${headers[key]}, ${key} was ${data[key]}`
        );
      });
    })
    .then('a $method request is made to "$url" with the body "$body"', function(method, url, data) {
      const request = api.server.history[api.server.history.length - 2];
      assert.equal(
        request.method,
        method,
        `Expected the request method to be ${method}, was ${request.method}`
      );
      assert.equal(request.url, url, `Expected the request url to be ${url}, was ${request.url}`);
      const body = request.requestBody;
      assert.equal(body, data, `Expected the request body to be ${data}, was ${body}`);
    })
    .then('a $method request is made to "$url" with no body', function(method, url) {
      const request = api.server.history[api.server.history.length - 2];
      assert.equal(
        request.method,
        method,
        `Expected the request method to be ${method}, was ${request.method}`
      );
      assert.equal(request.url, url, `Expected the request url to be ${url}, was ${request.url}`);
      const body = request.requestBody;
      assert.equal(body, null, `Expected the request body to be null, was ${body}`);
    })

    .then('a $method request is made to "$url"', function(method, url) {
      const request = api.server.history[api.server.history.length - 2];
      assert.equal(
        request.method,
        method,
        `Expected the request method to be ${method}, was ${request.method}`
      );
      assert.equal(request.url, url, `Expected the request url to be ${url}, was ${request.url}`);
    })
    .then('the last $method request was made to "$url"', function(method, url) {
      const request = lastRequest(method);
      assert.equal(
        request.method,
        method,
        `Expected the request method to be ${method}, was ${request.method}`
      );
      assert.equal(request.url, url, `Expected the request url to be ${url}, was ${request.url}`);
    })
    .then('the last $method request was made to "$url" with the body from yaml\n$yaml', function(
      method,
      url,
      data
    ) {
      const request = lastRequest(method);
      assert.ok(request, `Expected a ${method} request`);
      assert.equal(
        request.method,
        method,
        `Expected the request method to be ${method}, was ${request.method}`
      );
      assert.equal(request.url, url, `Expected the request url to be ${url}, was ${request.url}`);
      const body = JSON.parse(request.requestBody);
      Object.keys(data).forEach(function(key, i, arr) {
        assert.deepEqual(
          body[key],
          data[key],
          `Expected the payload to contain ${key} equaling ${data[key]}, ${key} was ${body[key]}`
        );
      });
    })
    .then('the last $method requests were like yaml\n$yaml', function(method, data) {
      const requests = api.server.history.reverse().filter(function(item) {
        return item.method === method;
      });
      data.reverse().forEach(function(item, i, arr) {
        assert.equal(
          requests[i].url,
          item,
          `Expected the request url to be ${item}, was ${requests[i].url}`
        );
      });
    });
}
