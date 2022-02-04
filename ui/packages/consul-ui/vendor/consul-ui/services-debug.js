(services =>
  services({
    'route:application': {
      class: 'consul-ui/routing/application-debug',
    },
    'service:intl': {
      class: 'consul-ui/services/i18n-debug',
    },
  }))(
  (
    json,
    data = typeof document !== 'undefined' ? document.currentScript.dataset : module.exports
  ) => {
    data[`services`] = JSON.stringify(json);
  }
);
