/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export function initialize(appInstance) {
  const intl = appInstance.lookup('service:intl');
  
  // Pre-load HDS translations to avoid updating intl during render
  // This prevents the "Assertion Failed: You attempted to update `_intls`" error
  // that occurs when HDS components try to add translations during rendering
  try {
    // HDS components will automatically register their translations
    // We just need to ensure the intl service is ready before rendering
    if (intl && typeof intl.addTranslations === 'function') {
      // Trigger any pending translation additions before app renders
      // by accessing the locale which forces initialization
      intl.locale;
    }
  } catch (e) {
    // Silently fail if there's an issue - translations will load normally
    console.warn('HDS translations pre-load warning:', e);
  }
}

export default {
  name: 'hds-translations',
  initialize,
};

// Made with Bob
