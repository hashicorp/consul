/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Adapter from './application';
import { get } from '@ember/object';

// Intentions have different namespacing to the rest of the UI in that the don't
// have a Namespace property, the DestinationNS is essentially its namespace.
// Listing of intentions still requires the `ns` query string parameter which
// will give us all the intentions that have the `ns` as either the SourceNS or
// the DestinationNS.

// TODO: Investigate the above to see if we should only list intentions with
// Source/Destination in the currently selected nspace or leave as is.
export default class IntentionAdapter extends Adapter {
  requestForQuery(request, { dc, ns, partition, filter, index, uri }) {
    return request`
      GET /v1/connect/intentions?${{ dc }}
      X-Request-ID: ${uri}${
      typeof filter !== 'undefined'
        ? `
      X-Range: ${filter}`
        : ``
    }

      ${{
        partition,
        ns: '*',
        index,
        filter,
      }}
    `;
  }

  requestForQueryRecord(request, { dc, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }

    // get the information we need from the id, which has been previously
    // encoded
    if (id.match(/^peer:/)) {
      // id indicates we are dealing with a peered intention - handle this with
      // different query-param for source - we need to prepend the peer
      const [
        peerIdentifier,
        SourcePeer,
        SourceNS,
        SourceName,
        DestinationPartition,
        DestinationNS,
        DestinationName,
      ] = id.split(':').map(decodeURIComponent);

      return request`
      GET /v1/connect/intentions/exact?${{
        source: `${peerIdentifier}:${SourcePeer}/${SourceNS}/${SourceName}`,
        destination: `${DestinationPartition}/${DestinationNS}/${DestinationName}`,
        dc: dc,
      }}
      Cache-Control: no-store

      ${{ index }}
    `;
    } else {
      const [
        SourcePartition,
        SourceNS,
        SourceName,
        DestinationPartition,
        DestinationNS,
        DestinationName,
      ] = id.split(':').map(decodeURIComponent);

      return request`
      GET /v1/connect/intentions/exact?${{
        source: `${SourcePartition}/${SourceNS}/${SourceName}`,
        destination: `${DestinationPartition}/${DestinationNS}/${DestinationName}`,
        dc: dc,
      }}
      Cache-Control: no-store

      ${{ index }}
    `;
    }
  }

  requestForCreateRecord(request, serialized, data) {
    const body = {
      SourceName: serialized.SourceName,
      DestinationName: serialized.DestinationName,
      SourceNS: serialized.SourceNS,
      DestinationNS: serialized.DestinationNS,
      SourcePartition: serialized.SourcePartition,
      DestinationPartition: serialized.DestinationPartition,
      SourceType: serialized.SourceType,
      Meta: serialized.Meta,
      Description: serialized.Description,
    };

    // only send the Action if we have one
    if (get(serialized, 'Action.length')) {
      body.Action = serialized.Action;
    } else {
      // otherwise only send Permissions if we have them
      if (serialized.Permissions) {
        body.Permissions = serialized.Permissions;
      }
    }
    return request`
      PUT /v1/connect/intentions/exact?${{
        source: `${data.SourcePartition}/${data.SourceNS}/${data.SourceName}`,
        destination: `${data.DestinationPartition}/${data.DestinationNS}/${data.DestinationName}`,
        dc: data.Datacenter,
      }}

      ${body}
    `;
  }

  requestForUpdateRecord(request, serialized, data) {
    // you can no longer save Destinations
    delete serialized.DestinationName;
    delete serialized.DestinationNS;
    delete serialized.DestinationPartition;
    return this.requestForCreateRecord(...arguments);
  }

  requestForDeleteRecord(request, serialized, data) {
    return request`
      DELETE /v1/connect/intentions/exact?${{
        source: `${data.SourcePartition}/${data.SourceNS}/${data.SourceName}`,
        destination: `${data.DestinationPartition}/${data.DestinationNS}/${data.DestinationName}`,
        dc: data.Datacenter,
      }}
    `;
  }
}
