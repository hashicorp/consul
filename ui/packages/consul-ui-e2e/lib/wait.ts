/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

/** Poll `fn` until it resolves truthy or the timeout elapses. */
export async function waitFor<T>(
  fn: () => Promise<T>,
  opts: { timeoutMs?: number; intervalMs?: number; label?: string } = {},
): Promise<T> {
  const { timeoutMs = 30_000, intervalMs = 1_000, label = 'condition' } = opts;
  const deadline = Date.now() + timeoutMs;
  let lastErr: unknown;

  while (Date.now() < deadline) {
    try {
      const result = await fn();
      if (result) return result;
    } catch (err) {
      lastErr = err;
    }
    await sleep(intervalMs);
  }

  const suffix = lastErr instanceof Error ? ` (last error: ${lastErr.message})` : '';
  throw new Error(`Timed out after ${timeoutMs}ms waiting for ${label}${suffix}`);
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
