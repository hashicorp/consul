/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (css) => {
  /*%visually-hidden {*/
  return css`
    @keyframes visually-hidden {
      100% {
        position: absolute;
        overflow: hidden;
        clip: rect(0 0 0 0);
        width: 1px;
        height: 1px;
        margin: -1px;
        padding: 0;
        border: 0;
      }
    }
  `;
  /*}*/
};
