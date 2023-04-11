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
    };
    if (typeof this._headers.body.index !== 'undefined') {
      // this should probably be in a response
      this._headers['content-type'] = 'text/event-stream';
    }
  }
  headers() {
    return this._headers;
  }
  open(xhr) {
    this._xhr = xhr;
    this.dispatchEvent({ type: 'open' });
  }
  respond(data) {
    this.dispatchEvent({ type: 'message', data: data });
  }
  error(error) {
    // if the xhr was aborted (status = 0)
    // and this requests was aborted with a different status
    // switch the status
    if (error.statusCode === 0 && typeof this.statusCode !== 'undefined') {
      error.statusCode = this.statusCode;
    }
    this.dispatchEvent({ type: 'error', error: error });
  }
  close() {
    this.dispatchEvent({ type: 'close' });
  }
  connection() {
    return this._xhr;
  }
  abort(statusCode = 0) {
    if (this.headers()['content-type'] === 'text/event-stream') {
      this.statusCode = statusCode;
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
