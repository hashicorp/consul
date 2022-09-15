const not = `(n't| not)?`;
export default function (scenario, assert, lastNthRequest) {
  // lastNthRequest should return a
  // {
  //   method: '',
  //   requestBody: '',
  //   requestHeaders: ''
  // }
  scenario
    .then('the last $method requests included from yaml\n$yaml', function (method, data) {
      const requests = lastNthRequest(null, method);
      const a = new Set(data);
      const b = new Set(
        requests.map(function (item) {
          return item.url;
        })
      );
      const diff = new Set(
        [...a].filter(function (item) {
          return !b.has(item);
        })
      );
      assert.equal(diff.size, 0, `Expected requests "${[...diff].join(', ')}"`);
    })
    .then(`a $method request was${not} made to "$endpoint"`, function (method, negative, url) {
      const isNegative = typeof negative !== 'undefined';
      const requests = lastNthRequest(null, method);
      const request = requests.some(function (item) {
        return method === item.method && url === item.url;
      });
      if (isNegative) {
        assert.notOk(request, `Didn't expect a ${method} request url to ${url}`);
      } else {
        assert.ok(request, `Expected a ${method} request url to ${url}`);
      }
    })
    .then('a $method request was made to "$endpoint" with no body', function (method, url) {
      const requests = lastNthRequest(null, method);
      const request = requests.find(function (item) {
        return method === item.method && url === item.url;
      });
      assert.equal(
        request.requestBody,
        null,
        `Expected the request body to be null, was ${request.requestBody}`
      );
    })
    .then(
      'a $method request was made to "$endpoint" with the body "$body"',
      function (method, url, body) {
        const requests = lastNthRequest(null, method);
        const request = requests.find(function (item) {
          return method === item.method && url === item.url;
        });
        assert.ok(request, `Expected a ${method} request url to ${url} with the body "${body}"`);
      }
    )
    .then(
      'a $method request was made to "$endpoint" from yaml\n$yaml',
      function (method, url, yaml) {
        const requests = lastNthRequest(null, method);
        const request = requests.find(function (item) {
          return method === item.method && url === item.url;
        });
        let data = yaml.body || {};
        const body = JSON.parse(request.requestBody);
        Object.keys(data).forEach(function (key, i, arr) {
          assert.deepEqual(
            body[key],
            data[key],
            `Expected the payload to contain ${key} equaling ${JSON.stringify(
              data[key]
            )}, ${key} was ${JSON.stringify(body[key])}`
          );
        });
        data = yaml.headers || {};
        const headers = request.requestHeaders;
        Object.keys(data).forEach(function (key, i, arr) {
          assert.deepEqual(
            headers[key],
            data[key],
            `Expected the payload to contain ${key} equaling ${JSON.stringify(
              data[key]
            )}, ${key} was ${JSON.stringify(headers[key])}`
          );
        });
      }
    )
    .then(
      'a $method request was made to "$endpoint" without properties from yaml\n$yaml',
      function (method, url, properties) {
        const requests = lastNthRequest(null, method);
        const request = requests.find(function (item) {
          return method === item.method && url === item.url;
        });
        const body = JSON.parse(request.requestBody);
        properties.forEach(function (key, i, arr) {
          assert.equal(
            typeof body[key],
            'undefined',
            `Expected payload to not have a ${key} property`
          );
        });
      }
    );
}
