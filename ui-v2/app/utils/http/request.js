export default class {
  constructor(method, url, headers, xhr) {
    this._xhr = xhr;
    this._url = url;
    this._method = method;
    this._headers = headers;
    this._headers = {
      ...headers,
      'content-type': 'application/json',
      'x-request-id': `${this._method} ${this._url}?${JSON.stringify(headers.body)}`,
    };
    if (typeof this._headers.body.index !== 'undefined') {
      // this should probably be in a response
      this._headers['content-type'] = 'text/event-stream';
    }
  }
  headers() {
    return this._headers;
  }
  getId() {
    return this._headers['x-request-id'];
  }
  abort() {
    this._xhr.abort();
  }
  connection() {
    return this._xhr;
  }
}
