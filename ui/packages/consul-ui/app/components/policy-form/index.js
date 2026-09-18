/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import FormComponent from '../form-component/index';

export default FormComponent.extend({
  type: 'policy',
  name: 'policy',
  allowIdentity: true,
  classNames: ['policy-form'],
});
