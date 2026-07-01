/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';

// Column definitions for the auth methods table. Sorting and searching continue
// to be driven by the toolbar, so columns are non-sortable here; cell rendering
// lives in the template's :row block.
const COLUMNS = [
  { label: 'Name' },
  { label: 'External source' },
  { label: 'Internal name' },
  {
    label: 'Max time to live',
    tooltip:
      'Maximum Time to Live: the maximum life of any token created by this auth method',
  },
];

/**
 * Consul::AuthMethod::List
 *
 * Auth methods specific configuration for the generic Consul::DataTable. It
 * supplies the columns and renders each row's cells via the :row block. It owns
 * no sorting / pagination / data fetching; it receives already
 * fetched/filtered/searched `@items`. The list has no per-row actions (each row
 * links through to the auth method's show page).
 */
export default class ConsulAuthMethodList extends Component {
  columns = COLUMNS;
}
