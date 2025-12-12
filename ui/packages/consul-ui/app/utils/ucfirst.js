/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (str) {
  return `${str.substr(0, 1).toUpperCase()}${str.substr(1)}`;
}
