/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { helper } from '@ember/component/helper';

export default helper(function formatTime([params], hash) {
  let day, hour, minute, seconds;
  seconds = Math.floor(params / 1000);
  minute = Math.floor(seconds / 60);
  seconds = seconds % 60;
  hour = Math.floor(minute / 60);
  minute = minute % 60;
  day = Math.floor(hour / 24);
  hour = hour % 24;
  const time = {
    day: day,
    hour: hour,
    minute: minute,
    seconds: seconds,
  };

  switch (true) {
    case time.day !== 0:
      return time.day + 'd';
    case time.hour !== 0:
      return time.hour + 'h';
    case time.minute !== 0:
      return time.minute + 'm';
    default:
      return time.seconds + 's';
  }
});
