export default function(Settings) {
  // const settings = Settings.create();
  const settings = {
    get: function() {
      return '';
    },
  };
  return function(url, dc) {
    var data = {
      dc: dc,
      token: settings.get('token'),
    };
    var params = Object.keys(data)
      .reduce(function(prev, item, i, arr) {
        if (data[item] != null && data[item] != '') {
          prev = prev.concat([item + '=' + data[item]]);
        }
        return prev;
      }, [])
      .join('&');
    if (url.indexOf('?') !== -1) {
      // If our url has existing params
      url += '&';
    } else {
      // Our url doesn't have params
      url += '?';
    }
    return url + params;
  };
}
