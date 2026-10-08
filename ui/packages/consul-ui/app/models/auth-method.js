/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Model, { attr } from '@ember-data/model';
import parse from 'parse-duration';

export const PRIMARY_KEY = 'uid';
export const SLUG_KEY = 'Name';

export default class AuthMethod extends Model {
  @attr('string') uid;
  @attr('string') Name;

  @attr('string') Datacenter;
  @attr('string') Namespace;
  @attr('string') Partition;
  @attr('string', { defaultValue: () => '' }) Description;
  @attr('string', { defaultValue: () => '' }) DisplayName;
  @attr('string', { defaultValue: () => 'local' }) TokenLocality;
  @attr('string', { defaultValue: () => '' }) TokenNameFormat;
  @attr('string') Type;
  @attr() NamespaceRules;
  get MethodName() {
    return this.DisplayName || this.Name;
  }
  @attr() Config;
  @attr('string') MaxTokenTTL;
  @attr('number') CreateIndex;
  @attr('number') ModifyIndex;
  @attr() Datacenters; // string[]
  @attr() meta; // {}

  get TokenTTL() {
    return parse(this.MaxTokenTTL);
  }

  // The API returns MaxTokenTTL as a raw Go duration string (e.g. "24h0m0s",
  // "5m20s") which reads inconsistently. Present it as a single, readable
  // duration using short (3+ letter) unit labels (e.g. "1 day", "5 mins 20
  // secs", "1 hr 30 mins"). Every non-zero component the API sends is kept —
  // including any sub-second remainder — so no time data is lost; only zero
  // components are dropped. Falls back to the raw value if it can't be parsed.
  get MaxTokenTTLFormatted() {
    let remaining = this.TokenTTL;
    if (!remaining) {
      return this.MaxTokenTTL;
    }
    const units = [
      { ms: 86400000, one: 'day', many: 'days' },
      { ms: 3600000, one: 'hr', many: 'hrs' },
      { ms: 60000, one: 'min', many: 'mins' },
      { ms: 1000, one: 'sec', many: 'secs' },
      { ms: 1, one: 'ms', many: 'ms' },
    ];
    const parts = [];
    units.forEach(({ ms, one, many }) => {
      const value = Math.floor(remaining / ms);
      if (value > 0) {
        parts.push(`${value} ${value === 1 ? one : many}`);
        remaining -= value * ms;
      }
    });
    return parts.length ? parts.join(' ') : this.MaxTokenTTL;
  }
}
