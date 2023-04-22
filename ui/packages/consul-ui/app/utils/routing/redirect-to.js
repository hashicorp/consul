/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (to, route) {
  return function (model, transition) {
    const parent = this.routeName.split('.').slice(0, -1).join('.');
    this.replaceWith(`${parent}.${to}`, model);
  };
}
