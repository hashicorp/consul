/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (arr) {
  if (Array.isArray(arr)) {
    return arr
      .filter(function (item) {
        // Just incase, don't save any models that aren't saved
        return !item?.isNew;
      })
      .map(function (item) {
        return {
          ID: item?.ID,
          Name: item?.Name,
        };
      });
  }
}
