(services => services({
  "route:application": {
    "class": "consul-ui/routing/application-debug"
  },
  "service:intl": {
    "class": "consul-ui/services/i18n-debug"
  }
}))(
  (json, data = document.currentScript.dataset) => {
    const appNameJS = data.appName.split('-')
      .map((item, i) => i ? `${item.substr(0, 1).toUpperCase()}${item.substr(1)}` : item)
      .join('');
    data[`${appNameJS}Services`] = JSON.stringify(json);
  }
);
