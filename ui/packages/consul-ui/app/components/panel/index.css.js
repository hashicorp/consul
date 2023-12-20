/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default (css) => css`
  .panel {
    --padding-x: 14px;
    --padding-y: 14px;
  }
  .panel {
    position: relative;
  }
  .panel-separator {
    margin: 0;
  }

  .panel {
    --tone-border: var(--token-color-palette-neutral-300);
    border: var(--decor-border-100);
    border-radius: var(--decor-radius-200);
    box-shadow: var(--token-surface-high-box-shadow);
  }
  .panel-separator {
    border: 0;
    border-top: var(--decor-border-100);
  }
  .panel {
    color: var(--token-color-foreground-strong);
    background-color: var(--token-color-surface-primary);
  }
  .panel,
  .panel-separator {
    border-color: var(--tone-border);
  }
`;
