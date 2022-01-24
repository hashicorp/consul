import { runInDebug } from '@ember/debug';
import require from 'require';
import merge from 'deepmerge';

const doc = document;

export const services = merge.all(
  [...doc.querySelectorAll(`script[data-services]`)].map($item =>
    JSON.parse($item.dataset[`services`])
  )
);

const inject = function(container, obj) {
  // inject all the things
  Object.entries(obj).forEach(([key, value]) => {
    switch (true) {
      case typeof value.class === 'string':
        if (require.has(value.class)) {
          container.register(
            key.replace('auth-provider:', 'torii-provider:'),
            require(value.class).default
          );
        } else {
          throw new Error(`Unable to locate '${value.class}'`);
        }
        break;
    }
  });
};
export default {
  name: 'container',
  initialize(application) {
    inject(application, services);

    const container = application.lookup('service:container');
    // find all the services and add their classes to the container so we can
    // look instances up by class afterwards as we then resolve the
    // registration for each of these further down this means that any top
    // level code for these services is executed, this is most useful for
    // making sure any annotation type decorators are executed.
    // For now we only want repositories, so only look for those for the moment
    let repositories = container
      .get('container-debug-adapter:main')
      .catalogEntriesByType('service')
      .filter(item => item.startsWith('repository/') || item === 'ui-config');

    // during testing we get -test files in here, filter those out but only in debug envs
    runInDebug(() => (repositories = repositories.filter(item => !item.endsWith('-test'))));

    // 'service' service is not returned by catalogEntriesByType, possibly
    // related to pods and the service being called 'service':
    // https://github.com/ember-cli/ember-resolver/blob/c07287af17766bfd3acf390f867fea17686f77d2/addon/resolvers/classic/container-debug-adapter.js#L80
    // so push it on the end
    repositories.push('repository/service');
    //
    repositories.forEach(item => {
      const key = `service:${item}`;
      container.set(key, container.resolveRegistration(key));
    });
  },
};
