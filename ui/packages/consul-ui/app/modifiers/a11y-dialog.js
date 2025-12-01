/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';
import A11yDialog from 'a11y-dialog';

/**
 * A custom modifier to set up and manage an a11y-dialog instance.
 *
 * This modifier creates an A11yDialog instance, sets up event listeners,
 * and handles cleanup when the element is destroyed.
 *
 * Usage:
 *   {{a11y-dialog
 *     onShow=this.handleShow
 *     onHide=this.handleHide
 *     onSetup=(fn this.handleSetup)
 *     autoOpen=@open
 *   }}
 *
 * @param {Function} onShow - Callback when dialog is shown
 * @param {Function} onHide - Callback when dialog is hidden
 * @param {Function} onSetup - Callback with dialog instance after setup
 * @param {Boolean} autoOpen - Whether to open the dialog on setup
 */
export default modifier((element, _positional, { onShow, onHide, onSetup, autoOpen }) => {
  // Create the A11yDialog instance
  const dialog = new A11yDialog(element);

  // Set up event listeners
  if (typeof onShow === 'function') {
    dialog.on('show', () => {
      onShow({ target: element });
    });
  }

  if (typeof onHide === 'function') {
    dialog.on('hide', () => {
      onHide({ target: element });
    });
  }

  // Call setup callback with dialog instance
  if (typeof onSetup === 'function') {
    onSetup(dialog);
  }

  // Auto-open if requested
  if (autoOpen) {
    dialog.show();
  }

  // Return cleanup function
  return () => {
    if (dialog) {
      dialog.destroy();
    }
  };
});
