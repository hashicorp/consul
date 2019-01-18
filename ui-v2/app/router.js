import EmberRouter from '@ember/routing/router';
import config from './config/environment';

const Router = EmberRouter.extend({
  location: config.locationType,
  rootURL: config.rootURL,
});
export const routes = {
  dc: {
    _options: { path: ':dc' },
    services: {
      _options: { path: '/services' },
      show: {
        _options: { path: '/:name' },
      },
    },
    nodes: {
      _options: { path: '/nodes' },
      show: {
        _options: { path: '/:name' },
      },
    },
    intentions: {
      _options: { path: '/intentions' },
      edit: {
        _options: { path: '/:id' },
      },
      create: {
        _options: { path: '/create' },
      },
    },
    kv: {
      _options: { path: '/kv' },
      folder: {
        _options: { path: '/*key' },
      },
      edit: {
        _options: { path: '/*key/edit' },
      },
      create: {
        _options: { path: '/*key/create' },
      },
      'root-create': {
        _options: { path: '/create' },
      },
    },
    acls: {
      _options: { path: '/acls' },
      edit: {
        _options: { path: '/:id' },
      },
      create: {
        _options: { path: '/create' },
      },
      policies: {
        _options: { path: '/policies' },
        edit: {
          _options: { path: '/:id' },
        },
        create: {
          _options: { path: '/create' },
        },
      },
      tokens: {
        _options: { path: '/tokens' },
        edit: {
          _options: { path: '/:id' },
        },
        create: {
          _options: { path: '/create' },
        },
      },
    },
  },
  index: {
    _options: { path: '/' },
  },
  notfound: {
    _options: { path: '/*path' },
  },
};
const map = function(routes) {
  const keys = Object.keys(routes);
  keys.forEach((item, i) => {
    if (item === '_options') {
      return;
    }
    const options = routes[item]._options;
    this.route(item, options, function() {
      if (Object.keys(routes[item]).length > 1) {
        map.bind(this)(routes[item]);
      }
    });
  });
  if (typeof routes.index === 'undefined') {
    routes.index = {
      _options: {
        path: '',
      },
    };
  }
};
Router.map(function() {
  map.bind(this)(routes);
});
// Router.map(function() {
//   // Our parent datacenter resource sets the namespace
//   // for the entire application
//   this.route('dc', { path: '/:dc' }, function() {
//     // Services represent a consul service
//     this.route('services', { path: '/services' }, function() {
//       // Show an individual service
//       this.route('show', { path: '/*name' });
//     });
//     // Nodes represent a consul node
//     this.route('nodes', { path: '/nodes' }, function() {
//       // Show an individual node
//       this.route('show', { path: '/:name' });
//     });
//     // Intentions represent a consul intention
//     this.route('intentions', { path: '/intentions' }, function() {
//       this.route('edit', { path: '/:id' });
//       this.route('create', { path: '/create' });
//     });
//     // Key/Value
//     this.route('kv', { path: '/kv' }, function() {
//       this.route('folder', { path: '/*key' });
//       this.route('edit', { path: '/*key/edit' });
//       this.route('create', { path: '/*key/create' });
//       this.route('root-create', { path: '/create' });
//     });
//     // ACLs
//     this.route('acls', { path: '/acls' }, function() {
//       this.route('edit', { path: '/:id' });
//       this.route('create', { path: '/create' });
//       this.route('policies', { path: '/policies' }, function() {
//         this.route('edit', { path: '/:id' });
//         this.route('create', { path: '/create' });
//       });
//       this.route('tokens', { path: '/tokens' }, function() {
//         this.route('edit', { path: '/:id' });
//         this.route('create', { path: '/create' });
//       });
//     });
//   });

//   // Shows a datacenter picker. If you only have one
//   // it just redirects you through.
//   this.route('index', { path: '/' });

//   // The settings page is global.
//   // this.route('settings', { path: '/settings' });
//   this.route('notfound', { path: '/*path' });
// });
export default Router;
