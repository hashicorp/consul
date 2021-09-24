import Adapter from './application';

// Blocking query support for partitions is currently disabled
export default class PartitionAdapter extends Adapter {
  async requestForQuery(request, { ns, dc, index }) {
    const respond = await request`
      GET /v1/partitions?${{ dc }}

      ${{ index }}
    `;
    await respond((headers, body) => delete headers['x-consul-index']);
    return respond;
  }
  // TODO: Not used until we do Partition CRUD
  async requestForQueryRecord(request, { ns, dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    const respond = await request`
      GET /v1/partition/${id}?${{ dc }}

      ${{ index }}
    `;
    await respond((headers, body) => delete headers['x-consul-index']);
    return respond;
  }
}
