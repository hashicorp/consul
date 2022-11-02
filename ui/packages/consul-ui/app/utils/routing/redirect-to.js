export default function (to, route) {
  return function (model, transition) {
    const parent = this.routeName.split('.').slice(0, -1).join('.');
    this.replaceWith(`${parent}.${to}`, model);
  };
}
