const getNodesByType = function(nodes = {}, type) {
  return Object.values(nodes).filter(item => item.Type === type);
};
const findResolver = function(resolvers, service, nspace = 'default', partition = 'default', dc) {
  if (typeof resolvers[service] === 'undefined') {
    resolvers[service] = {
      ID: `${service}.${nspace}.${partition}.${dc}`,
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
    const types = ['Datacenter', 'Partition', 'Namespace', 'Service', 'Subset'];
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
    // splitters have a service.nspace as a name
    // do the reverse dance to ensure we don't mess up any
    // service names with dots in them
    const temp = item.Name.split('.');
    temp.reverse();
    temp.shift();
    temp.shift();
    temp.reverse();
    return {
      ...item,
      ID: `splitter:${item.Name}`,
      Name: temp.join('.'),
    };
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
export const getResolvers = function(
  dc,
  partition = 'default',
  nspace = 'default',
  targets = {},
  nodes = {}
) {
  const resolvers = {};
  // make all our resolver nodes
  Object.values(nodes)
    .filter(item => item.Type === 'resolver')
    .forEach(function(item) {
      const parts = item.Name.split('.');
      let subset;
      // this will leave behind the service.name.nspace.partition.dc even if the service name contains a dot
      if (parts.length > 4) {
        subset = parts.shift();
      }
      parts.reverse();
      // slice off from dc.partition.nspace onwards leaving the potentially dot containing service name
      // const nodeDc =
      parts.shift();
      // const nodePartition =
      parts.shift();
      // const nodeNspace =
      parts.shift();
      // if it does contain a dot put it back to the correct order
      parts.reverse();
      const service = parts.join('.');
      const resolver = findResolver(resolvers, service, nspace, partition, dc);
      let failovers;
      if (typeof item.Resolver.Failover !== 'undefined') {
        // figure out what type of failover this is
        failovers = getAlternateServices(item.Resolver.Failover.Targets, item.Name);
      }
      if (subset) {
        const child = {
          Subset: true,
          ID: item.Name,
          Name: subset,
        };
        if (typeof failovers !== 'undefined') {
          child.Failover = failovers;
        }
        resolver.Children.push(child);
      } else {
        if (typeof failovers !== 'undefined') {
          resolver.Failover = failovers;
        }
      }
    });
  Object.values(targets).forEach(target => {
    // Failovers don't have a specific node
    if (typeof nodes[`resolver:${target.ID}`] !== 'undefined') {
      // We use this to figure out whether this target is a redirect target
      const alternate = getAlternateServices([target.ID], `service.${nspace}.${partition}.${dc}`);
      // as Failovers don't make it here, we know anything that has alternateServices
      // must be a redirect
      if (alternate.Type !== 'Service') {
        // find the already created resolver
        const resolver = findResolver(resolvers, target.Service, nspace, partition, dc);
        // and add the redirect as a child, redirects are always children
        const child = {
          Redirect: alternate.Type,
          ID: target.ID,
          Name: target[alternate.Type],
        };
        // redirects can then also have failovers
        // so it this one does, figure out what type they are and add them
        // to the redirect
        if (typeof nodes[`resolver:${target.ID}`].Resolver.Failover !== 'undefined') {
          child.Failover = getAlternateServices(
            nodes[`resolver:${target.ID}`].Resolver.Failover.Targets,
            target.ID
          );
        }
        resolver.Children.push(child);
      }
    }
  });
  return Object.values(resolvers);
};
export const createRoute = function(route, router, uid) {
  return {
    ...route,
    Default: route.Default || typeof route.Definition.Match === 'undefined',
    ID: `route:${router}-${uid(route.Definition)}`,
  };
};
