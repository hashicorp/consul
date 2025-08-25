/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import {
  validatePresence,
  validateLength,
  validateFormat,
} from 'ember-changeset-validations/validators';

function validateNoSpaces(options = {}) {
  return function validate(key) {
    if (typeof key !== 'string') return true;

    if (/\s/.test(key)) {
      return options.message || 'Key cannot contain spaces';
    }

    return true;
  };
}

function validateNoLeadingSlash(options = {}) {
  return function validate(key) {
    if (typeof key !== 'string') return true;

    if (key.startsWith('/')) {
      return options.message || 'Key cannot start with a slash';
    }

    return true;
  };
}

function validateNoPathTraversal(options = {}) {
  return function validate(key) {
    if (typeof key !== 'string') return true;

    // Check for .. sequences (both raw and encoded)
    if (key.includes('..') || key.includes('%2e%2e') || key.includes('%2E%2E')) {
      return options.message || 'Key cannot contain path traversal sequences';
    }

    return true;
  };
}

function validateNoSuspiciousExtensions(options = {}) {
  const suspiciousExtensions = [
    'js',
    'php',
    'asp',
    'aspx',
    'jsp',
    'exe',
    'bat',
    'cmd',
    'sh',
    'py',
    'pl',
    'rb',
    'cgi',
    'html',
    'htm',
    'xml',
    'json',
  ];

  return function validate(key) {
    if (typeof key !== 'string') return true;

    const extension = key.split('.').pop()?.toLowerCase();
    if (extension && suspiciousExtensions.includes(extension)) {
      return options.message || `Key cannot have suspicious file extension: .${extension}`;
    }

    return true;
  };
}

export default {
  Key: [
    validatePresence(true),
    validateLength({ min: 1 }),
    validateFormat({
      regex: /^[a-zA-Z0-9,\-_./]+$/,
      message:
        'Key can only contain letters, numbers, commas, hyphens, underscores, periods, and forward slashes',
    }),
    validateNoSpaces(),
    validateNoLeadingSlash(),
    validateNoPathTraversal(),
    validateNoSuspiciousExtensions(),
  ],
};
