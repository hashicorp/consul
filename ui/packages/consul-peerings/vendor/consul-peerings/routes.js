(routes => routes({
  dc: {
    peers: {
      _options: { 
        path: '/peers'
      },
      index: {
        _options: {
          path: '/',
          queryParams: {
            sortBy: 'sort',
            state: 'state',
            searchproperty: {
              as: 'searchproperty',
              empty: [['Name', 'ID']],
            },
            search: {
              as: 'filter',
              replace: true,
            },
          },
        },
      },
    },
  },
}))(
  (json, data = (typeof document !== 'undefined' ? document.currentScript.dataset : module.exports)) => {
    data[`routes`] = JSON.stringify(json);
  }
);
