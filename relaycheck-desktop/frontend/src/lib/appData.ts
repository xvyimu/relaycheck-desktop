export interface LoadState {
  loaded: boolean;
  loading: boolean;
}

export interface RefreshableState {
  refresh: () => Promise<void>;
}

/** Shell can appear after system (health+status) is ready; inventory/ops hydrate after. */
export function appIsInitialLoading(system: LoadState, _inventory?: LoadState, _ops?: LoadState) {
  return !system.loaded;
}

export async function refreshAppData(
  system: RefreshableState,
  inventory: RefreshableState,
  ops: RefreshableState,
  modelUsage: RefreshableState,
) {
  await Promise.all([system.refresh(), inventory.refresh(), ops.refresh(), modelUsage.refresh()]);
}
