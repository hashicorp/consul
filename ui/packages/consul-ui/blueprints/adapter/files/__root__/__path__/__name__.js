import Adapter from './application';

export default class <%= classifiedModuleName %>Adapter extends Adapter {

  requestForQuery(request, { ns, dc, index }) {
    return request`
      GET /v1/<%= dasherizedModuleName %>?${{ dc }}

      ${{ index }}
    `;
  }

  requestForQueryRecord(request, { ns, dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/<%= dasherizedModuleName %>/${id}?${{ dc }}

      ${{ index }}
    `;
  }

}
