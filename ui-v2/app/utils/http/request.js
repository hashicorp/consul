import EventTarget from 'consul-ui/utils/dom/event-target/rsvp';
export default class extends EventTarget {
  constructor(method, url, headers) {
    super();
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
  open(xhr) {
    this._xhr = xhr;
    this.dispatchEvent({ type: 'open' });
  }
  respond(data) {
    this.dispatchEvent({ type: 'message', data: data });
  }
  error(error) {
    this.dispatchEvent({ type: 'error', error: error });
  }
  close() {
    this.dispatchEvent({ type: 'close' });
  }
  connection() {
    return this._xhr;
  }
  dispose() {
    if (this.headers()['content-type'] === 'text/event-stream') {
      const xhr = this.connection();
      // unsent and opened get aborted
      // headers and loading means wait for it
      // to finish for the moment
      if (xhr.readyState) {
        switch (xhr.readyState) {
          case 0:
          case 1:
            xhr.abort();
            break;
        }
      }
    }
  }
}
