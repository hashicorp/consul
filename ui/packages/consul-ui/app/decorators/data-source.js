import { runInDebug } from '@ember/debug';
import wayfarer from 'wayfarer';

const router = wayfarer();
const routes = {};
export default path => (target, propertyKey, desc) => {
  runInDebug(() => {
    routes[path] = { cls: target, method: propertyKey };
  });
  router.on(path, function(params, owner, request) {
    const container = owner.lookup('service:container');
    const instance = container.get(target);
    return configuration => desc.value.apply(instance, [params, configuration, request]);
  });
  return desc;
};
export const match = path => {
  return router.match(path);
};

runInDebug(() => {
  window.DataSourceRoutes = () => {
    // debug time only way to access the application and be able to lookup
    // services, don't use ConsulUi global elsewhere!
    const container = window.ConsulUi.__container__.lookup('service:container');
    const win = window.open('', '_blank');
    win.document.write(`
<body>
  <pre>
${Object.entries(routes)
  .map(([key, value]) => {
    let cls = container
      .keyForClass(value.cls)
      .split('/')
      .pop();
    cls = cls
      .split('-')
      .map(item => `${item[0].toUpperCase()}${item.substr(1)}`)
      .join('');
    return `${key}
      ${cls}Repository.${value.method}(params)

`;
  })
  .join('')}
  </pre>
</body>
    `);
    win.focus();
    return;
  };
});
