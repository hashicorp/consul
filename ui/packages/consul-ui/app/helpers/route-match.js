/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

export default helper(function routeMatch([item], hash) {
  const prop = ['Present', 'Exact', 'Prefix', 'Suffix', 'Contains', 'Regex'].find(
    (prop) => typeof item[prop] !== 'undefined'
  );

  let invertPrefix = item.Invert ? 'NOT ' : '';
  let ignoreCaseSuffix = item.IgnoreCase ? ' (case-insensitive)' : '';

  switch (prop) {
    case 'Present':
      return `${invertPrefix}present`;
    case 'Exact':
      return `${invertPrefix}exactly matching "${item.Exact}"${ignoreCaseSuffix}`;
    case 'Prefix':
      return `${invertPrefix}prefixed by "${item.Prefix}"${ignoreCaseSuffix}`;
    case 'Suffix':
      return `${invertPrefix}suffixed by "${item.Suffix}"${ignoreCaseSuffix}`;
    case 'Contains':
      return `${invertPrefix}containing "${item.Contains}"${ignoreCaseSuffix}`;
    case 'Regex':
      return `${invertPrefix}matching the regex "${item.Regex}"`;
  }
  return '';
});
