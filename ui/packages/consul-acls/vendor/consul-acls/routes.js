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
  (json, data = (typeof document !== 'undefined' ? document.currentScript.dataset : module.exports)) => {
    data[`routes`] = JSON.stringify(json);
  }
);
