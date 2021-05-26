export function initialize(container) {
  const env = container.lookup('service:env');
  if (env.var('CONSUL_NSPACES_ENABLED')) {
    // enable the nspace repo
    ['dc', 'settings', 'dc.intentions.edit', 'dc.intentions.create'].forEach(function(item) {
      container.inject(`route:${item}`, 'nspacesRepo', 'service:repository/nspace/enabled');
      container.inject(`route:nspace.${item}`, 'nspacesRepo', 'service:repository/nspace/enabled');
    });
    container.inject('route:application', 'nspacesRepo', 'service:repository/nspace/enabled');
  }
}

export default {
  initialize,
};
