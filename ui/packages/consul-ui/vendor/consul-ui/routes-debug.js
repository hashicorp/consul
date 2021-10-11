(routes => routes({
  ['oauth-provider-debug']: {
    _options: {
      path: '/oauth-provider-debug',
      queryParams: {
        redirect_uri: 'redirect_uri',
        response_type: 'response_type',
        scope: 'scope',
      },
    }
  },
}))(
  (json, data = document.currentScript.dataset) => {
    const appNameJS = data.appName.split('-')
      .map((item, i) => i ? `${item.substr(0, 1).toUpperCase()}${item.substr(1)}` : item)
      .join('');
    data[`${appNameJS}Routes`] = JSON.stringify(json);
  }
);
