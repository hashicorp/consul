/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (css) => css`
  *::before,
  *::after {
    display: inline-block;
    animation-play-state: paused;
    animation-fill-mode: forwards;
    animation-iteration-count: var(--icon-resolution, 1);
    vertical-align: text-top;
  }
  *::before {
    animation-name: var(--icon-name-start, var(--icon-name)),
      var(--icon-size-start, var(--icon-size, icon-000));
    background-color: var(--icon-color-start, var(--icon-color));
  }
  *::after {
    animation-name: var(--icon-name-end, var(--icon-name)),
      var(--icon-size-end, var(--icon-size, icon-000));
    background-color: var(--icon-color-end, var(--icon-color));
  }

  [style*='--icon-color-start']::before {
    color: var(--icon-color-start);
  }
  [style*='--icon-color-end']::after {
    color: var(--icon-color-end);
  }
  [style*='--icon-name-start']::before,
  [style*='--icon-name-end']::after {
    content: '';
  }

  @keyframes icon-000 {
    100% {
      width: 1.2em;
      height: 1.2em;
    }
  }
  @keyframes icon-100 {
    100% {
      width: 0.625rem; /* 10px */
      height: 0.625rem; /* 10px */
    }
  }
  @keyframes icon-200 {
    100% {
      width: 0.75rem; /* 12px */
      height: 0.75rem; /* 12px */
    }
  }
  @keyframes icon-300 {
    100% {
      width: 1rem; /* 16px */
      height: 1rem; /* 16px */
    }
  }
  @keyframes icon-400 {
    100% {
      width: 1.125rem; /* 18px */
      height: 1.125rem; /* 18px */
    }
  }
  @keyframes icon-500 {
    100% {
      width: 1.25rem; /* 20px */
      height: 1.25rem; /* 20px */
    }
  }
  @keyframes icon-600 {
    100% {
      width: 1.375rem; /* 22px */
      height: 1.375rem; /* 22px */
    }
  }
  @keyframes icon-700 {
    100% {
      width: 1.5rem; /* 24px */
      height: 1.5rem; /* 24px */
    }
  }
  @keyframes icon-800 {
    100% {
      width: 1.625rem; /* 26px */
      height: 1.625rem; /* 26px */
    }
  }
  @keyframes icon-900 {
    100% {
      width: 1.75rem; /* 28px */
      height: 1.75rem; /* 28px */
    }
  }
`;
