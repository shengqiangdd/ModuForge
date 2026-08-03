let pendingCount = $state(0);

export const globalLoading = {
  get value() { return pendingCount > 0; },
  inc() { pendingCount++; },
  dec() { pendingCount = Math.max(0, pendingCount - 1); },
  reset() { pendingCount = 0; },
};
