// prettier-ignore
export default function(JSON) {
  // Has to be a property on an object so babel knocks the indentation in
  return {
    _requestToJQueryAjaxHash: function(request) {
      let hash = {};

      hash.type = request.method;
      hash.url = request.url;
      hash.dataType = 'json';
      hash.context = this;

      if (request.data) {
        if (request.method !== 'GET') {
          hash.contentType = 'application/json; charset=utf-8';
          hash.data = JSON.stringify(request.data);
        } else {
          hash.data = request.data;
        }
      }

      let headers = request.headers;
      if (headers !== undefined) {
        hash.beforeSend = function(xhr) {
          Object.keys(headers).forEach((key) => xhr.setRequestHeader(key, headers[key]));
        };
      }

      return hash;
    }
  }._requestToJQueryAjaxHash;
}
