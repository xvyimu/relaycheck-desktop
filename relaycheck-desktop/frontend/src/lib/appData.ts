export interface LoadState {
  loaded: boolean;
  loading: boolean;
}

export interface RefreshableState {
  refresh: () => Promise<void>;
}

export function appIsInitialLoading(system: LoadState, inventory: LoadState, ops: LoadState) {
  return !system.loaded || !inventory.loaded || !ops.loaded;
}

export async function refreshAppData(
  system: RefreshableState,
  inventory: RefreshableState,
  ops: RefreshableState,
  modelUsage: RefreshableState,
) {
  await Promise.all([system.refresh(), inventory.refresh(), ops.refresh(), modelUsage.refresh()]);
}
