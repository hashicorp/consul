(routes => routes({
  dc: {
    partitions: {
      _options: {
        path: '/partitions',
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
        abilities: ['read partitions'],
      },
      edit: {
        _options: { path: '/:name' },
      },
      create: {
        _options: {
          template: 'dc/partitions/edit',
          path: '/create',
          abilities: ['create partitions'],
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
