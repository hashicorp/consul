(routes => routes({
  dc: {
    partitions: {
      _options: {
        path: '/partitions',
        abilities: ['read partitions'],
      },
      index: {
        _options: {
          path: '/',
          queryParams: {
            sortBy: 'sort',
            searchproperty: {
              as: 'searchproperty',
              empty: [['Name', 'Description']],
            },
            search: {
              as: 'filter',
              replace: true,
            },
          },
        },
      },
      edit: {
        _options: { path: '/:name' },
      },
      create: {
        _options: {
          template: '../edit',
          path: '/create',
          abilities: ['create partitions'],
        },
      },
    },
  },
}))(
  (json, data = (typeof document !== 'undefined' ? document.currentScript.dataset : module.exports)) => {
    data[`routes`] = JSON.stringify(json);
  }
);
