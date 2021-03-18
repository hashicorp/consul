import { runInDebug } from '@ember/debug';

export const walk = function(routes) {
  const route = routes.route || {};
  if (typeof route.index === 'undefined') {
    route.index = {
      path: '',
    };
  }
  const keys = Object.keys(route);
  keys.forEach((item, i) => {
    const options = {
      path: route[item].path,
    };
    let cb;
    if (typeof route[item].route !== 'undefined') {
      cb = function() {
        walk.apply(this, [route[item]]);
      };
    }
    this.route(item, options, cb);
  });
};

/**
 * Drop in for the Router.map callback e.g. `Router.map(walk(routes))`
 * Uses { walk } to recursively walk through a JSON object of routes
 * and use `Router.route` to define your routes for your ember application
 *
 * @param {object} routes - JSON representation of routes
 */
export default function(routes) {
  return function() {
    walk.apply(this, [routes]);
  };
}

export let dump = () => {};

runInDebug(() => {
  const indent = function(num) {
    return Array(num)
      .fill('  ', 0, num)
      .join('');
  };
  /**
   * String dumper to produce Router.map code
   * Uses { walk } to recursively walk through a JSON object of routes
   * to produce the code necessary to define your routes for your ember application
   *
   * @param {object} routes - JSON representation of routes
   * @example `console.log(dump(routes));`
   */
  dump = function(routes) {
    let level = 2;
    const obj = {
      out: '',
      route: function(name, options, cb) {
        this.out += `${indent(level)}this.route('${name}', ${JSON.stringify(options)}`;
        if (cb) {
          level++;
          this.out += `, function() {
`;
          cb.apply(this, []);
          level--;
          this.out += `${indent(level)}});
`;
        } else {
          this.out += ');';
        }
        this.out += `
`;
      },
    };
    walk.apply(obj, [routes]);
    return `Router.map(
  function() {
${obj.out}
  }
);`;
  };
});
