(routes => routes({
  dc: {
    acls: {
      tokens: {
        _options: {
          abilities: ['read tokens'],
        },
      },
    },
  },
}))(
  (json, data = document.currentScript.dataset) => {
    const appNameJS = data.appName.split('-')
      .map((item, i) => i ? `${item.substr(0, 1).toUpperCase()}${item.substr(1)}` : item)
      .join('');
    data[`${appNameJS}Routes`] = JSON.stringify(json);
  }
);
