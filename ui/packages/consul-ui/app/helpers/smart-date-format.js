/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';

const MILLISECONDS_IN_DAY = 1000 * 60 * 60 * 24;
const MILLISECONDS_IN_WEEK = MILLISECONDS_IN_DAY * 7;
/**
 * A function that returns if a date is within a week of the current time
 * @param {*} date - the date to check
 *
 */
function isNearDate(date) {
  const now = new Date();
  const aWeekAgo = +now - MILLISECONDS_IN_WEEK;
  const aWeekInFuture = +now + MILLISECONDS_IN_WEEK;

  return date >= aWeekAgo && date <= aWeekInFuture;
}

export default class SmartDateFormat extends Helper {
  @service temporal;
  @service intl;

  compute([value], hash) {
    return {
      isNearDate: isNearDate(value),
      relative: `${this.temporal.format(value)} ago`,
      friendly: this.intl.formatTime(value, {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: 'numeric',
        minute: 'numeric',
        second: 'numeric',
        hourCycle: 'h24',
      }),
    };
  }
}
