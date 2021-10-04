(services => services({
  "route:basic": {
    "class": "consul-ui/routing/route"
  },
  "service:intl": {
    "class": "consul-ui/services/i18n"
  },
  "service:state": {
    "class": "consul-ui/services/state-with-charts"
  },
  "auth-provider:oidc-with-url": {
    "class": "consul-ui/services/auth-providers/oauth2-code-with-url-provider"
  }
}))(
  (json, data = document.currentScript.dataset) => {
    const appNameJS = data.appName.split('-')
      .map((item, i) => i ? `${item.substr(0, 1).toUpperCase()}${item.substr(1)}` : item)
      .join('');
    data[`${appNameJS}Services`] = JSON.stringify(json);
  }
);
