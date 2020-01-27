const getNodesByType = function(nodes = {}, type) {
  return Object.values(nodes).filter(item => item.Type === type);
};
const findResolver = function(resolvers, service, nspace = 'default', dc) {
  if (typeof resolvers[service] === 'undefined') {
    resolvers[service] = {
      ID: `${service}.${nspace}.${dc}`,
      Name: service,
      Children: [],
    };
  }
  return resolvers[service];
};
export const getAlternateServices = function(targets, a) {
  let type;
  const Targets = targets.map(function(b) {
    // TODO: this isn't going to work past namespace for services
    // with dots in the name, but by the time that becomes an issue
    // we might have more data from the endpoint so we don't have to guess
    // right now the backend also doesn't support dots in service names
    const [aRev, bRev] = [a, b].map(item => item.split('.').reverse());
    const types = ['Datacenter', 'Namespace', 'Service', 'Subset'];
    return bRev.find(function(item, i) {
      const res = item !== aRev[i];
      if (res) {
        type = types[i];
      }
      return res;
    });
  });
  return {
    Type: type,
    Targets: Targets,
  };
};

export const getSplitters = function(nodes) {
  return getNodesByType(nodes, 'splitter').map(function(item) {
    // Splitters need IDs adding so we can find them in the DOM later
    item.ID = `splitter:${item.Name}`;
    return item;
  });
};
export const getRoutes = function(nodes, uid) {
  return getNodesByType(nodes, 'router').reduce(function(prev, item) {
    return prev.concat(
      item.Routes.map(function(route, i) {
        // Routes also have IDs added via createRoute
        return createRoute(route, item.Name, uid);
      })
    );
  }, []);
};
export const getResolvers = function(dc, nspace = 'default', targets = {}, nodes = {}) {
  const resolvers = {};
  Object.values(targets).forEach(target => {
    const node = nodes[`resolver:${target.ID}`];
    const resolver = findResolver(resolvers, target.Service, nspace, dc);
    // We use this to figure out whether this target is a redirect target
    const alternate = getAlternateServices([target.ID], `service.${nspace}.${dc}`);

    let failovers;
    // Figure out the failover type
    if (typeof node.Resolver.Failover !== 'undefined') {
      failovers = getAlternateServices(node.Resolver.Failover.Targets, target.ID);
    }
    switch (true) {
      // This target is a redirect
      case alternate.Type !== 'Service':
        resolver.Children.push({
          Redirect: true,
          ID: target.ID,
          Name: target[alternate.Type],
        });
        break;
      // This target is a Subset
      case typeof target.ServiceSubset !== 'undefined':
        resolver.Children.push({
          Subset: true,
          ID: target.ID,
          Name: target.ServiceSubset,
          Filter: target.Subset.Filter,
          ...(typeof failovers !== 'undefined'
            ? {
                Failover: failovers,
              }
            : {}),
        });
        break;
      // This target is just normal service that might have failovers
      default:
        if (typeof failovers !== 'undefined') {
          resolver.Failover = failovers;
        }
    }
  });
  return Object.values(resolvers);
};
export const createRoute = function(route, router, uid) {
  return {
    ...route,
    Default: typeof route.Definition.Match === 'undefined',
    ID: `route:${router}-${uid(route.Definition)}`,
  };
};
