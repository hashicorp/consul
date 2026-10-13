/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Server addresses (PeerServerAddresses) are plain strings, not records, so
// the only "property" to search on is the address itself.
export default {
  Address: (item) => item,
};
