import RepositoryService from 'consul-ui/services/repository';
import dataSource from 'consul-ui/decorators/data-source';

export default class PeerService extends RepositoryService {

  getModelName() {
    return 'peer';
  }

  @dataSource('/:partition/:ns/:dc/peers')
  async fetchAll({dc, ns, partition}, configuration, request) {
    return (await request`
      GET /v1/peerings?${{partition}}
    `)(
      (headers, body, cache) => {
        return {
          meta: {
            version: 2,
            interval: 10000,
            // uri: uri,
          },
          body: body.map(item => {
            return cache(
              {
                ...item,
                Datacenter: dc,
                Partition: partition,
              },
              uri => `peer:///${partition}/${ns}/${dc}/peer/${item.Name}`
            );
          })
        };
      });
  }
}
