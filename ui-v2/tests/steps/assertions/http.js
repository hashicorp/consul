// TODO: This entire file/steps need refactoring out so that they don't depend on order
// unless you specifically ask it to assert for order of requests
// this should also let us simplify the entire API for these steps
// an reword them to make more sense
export default function(scenario, assert, lastNthRequest) {
  // lastNthRequest should return a
  // {
  //   method: '',
  //   requestBody: '',
  //   requestHeaders: ''
  // }
  const assertRequest = function(request, method, url) {
    assert.equal(
      request.method,
      method,
      `Expected the request method to be ${method}, was ${request.method}`
    );
    assert.equal(request.url, url, `Expected the request url to be ${url}, was ${request.url}`);
  };
  scenario
    .then('a $method request is made to "$url" with the body from yaml\n$yaml', function(
      method,
      url,
      data
    ) {
      const request = lastNthRequest(1);
      assertRequest(request, method, url);
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
      const request = lastNthRequest(1);
      assertRequest(request, method, url);
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
      const request = lastNthRequest(1);
      assertRequest(request, method, url);
      assert.equal(
        request.requestBody,
        data,
        `Expected the request body to be ${data}, was ${request.requestBody}`
      );
    })
    .then('a $method request is made to "$url" with no body', function(method, url) {
      const request = lastNthRequest(1);
      assertRequest(request, method, url);
      assert.equal(
        request.requestBody,
        null,
        `Expected the request body to be null, was ${request.requestBody}`
      );
    })

    .then('a $method request is made to "$url"', function(method, url) {
      const request = lastNthRequest(1);
      assertRequest(request, method, url);
    })
    .then('the last $method request was made to "$url"', function(method, url) {
      const request = lastNthRequest(0, method);
      assertRequest(request, method, url);
    })
    .then('the last $method request was made to "$url" with the body from yaml\n$yaml', function(
      method,
      url,
      data
    ) {
      const request = lastNthRequest(0, method);
      const body = JSON.parse(request.requestBody);
      assert.ok(request, `Expected a ${method} request`);
      assertRequest(request, method, url);
      Object.keys(data).forEach(function(key, i, arr) {
        assert.deepEqual(
          body[key],
          data[key],
          `Expected the payload to contain ${key} equaling ${data[key]}, ${key} was ${body[key]}`
        );
      });
    })
    .then('the last $method requests were like yaml\n$yaml', function(method, data) {
      const requests = lastNthRequest(null, method);
      data.reverse().forEach(function(item, i, arr) {
        assert.equal(
          requests[i].url,
          item,
          `Expected the request url to be ${item}, was ${requests[i].url}`
        );
      });
    })
    .then('the last $method requests included from yaml\n$yaml', function(method, data) {
      const requests = lastNthRequest(null, method);
      const a = new Set(data);
      const b = new Set(
        requests.map(function(item) {
          return item.url;
        })
      );
      const diff = new Set(
        [...a].filter(function(item) {
          return !b.has(item);
        })
      );
      assert.equal(diff.size, 0, `Expected requests "${[...diff].join(', ')}"`);
    })
    .then('a $method request was made to "$url" from yaml\n$yaml', function(method, url, yaml) {
      const requests = lastNthRequest(null, method);
      const request = requests.find(function(item) {
        return method === item.method && url === item.url;
      });
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
    });
}
