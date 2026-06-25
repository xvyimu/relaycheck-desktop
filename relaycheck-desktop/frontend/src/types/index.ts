export type LineIconName =
  | "dashboard"
  | "channels"
  | "sites"
  | "accounts"
  | "checkins"
  | "balances"
  | "notifications"
  | "scan"
  | "settings"
  | "success"
  | "warning"
  | "danger"
  | "info";

export type TabKey =
  | "dashboard"
  | "channels"
  | "sites"
  | "accounts"
  | "checkins"
  | "balances"
  | "notifications"
  | "scan"
  | "settings";

export type NavItem = {
  key: TabKey;
  label: string;
  icon: LineIconName;
  description: string;
};

export type Summary = {
  localNewApiCount: number;
  importedChannelCount: number;
  identifiedChannelCount: number;
  accountCount: number;
  unreadNotifications: number;
};

export type StatusPayload = {
  productName: string;
  productVersion: string;
  buildTime: string;
  architecture: string;
  bindAddress: string;
  databasePath: string;
  backupDir: string;
  port: number;
  preferredPort?: number;
  portConflict?: boolean;
  networkProxy?: NetworkProxyStatus;
  scheduler?: SchedulerStatus;
  lastDiagnostics?: {
    overall: string;
    generatedAt: string;
    itemCount: number;
  };
  summary: Summary;
};

export type VersionCheckResult = {
  currentVersion: string;
  latestVersion?: string;
  updateAvailable: boolean;
  releaseUrl?: string;
  releaseNotes?: string;
  checkedAt: string;
  error?: string;
};

export type ExportResult = {
  fileName: string;
  sizeBytes: number;
  manifest: {
    version: string;
    exportedAt: string;
    productVersion: string;
    includes: { database: boolean; settings: boolean };
    databaseSize: number;
    settingCount: number;
  };
};

export type PortCheckResult = {
  port: number;
  available: boolean;
  inUse: boolean;
  inUseByPid?: number;
  error?: string;
};

export type AutoStartStatus = {
  enabled: boolean;
  supported: boolean;
  shortcutPath?: string;
  targetPath?: string;
  error?: string;
};

export type NetworkProxyStatus = {
  enabled: boolean;
  url: string;
  urlMasked: string;
  bypassLocal: boolean;
};

export type DiagnosticItem = {
  id: string;
  level: "success" | "info" | "warning" | "danger" | string;
  title: string;
  description: string;
  action?: string;
  solutionSteps?: string[];
  count?: number;
};

export type SystemDiagnostics = {
  generatedAt: string;
  overall: string;
  items: DiagnosticItem[];
};

export type SchedulerStatus = {
  generatedAt: string;
  jobs: SchedulerJobStatus[];
};

export type SchedulerJobStatus = {
  key: string;
  label: string;
  status: string;
  plannedRunKey?: string;
  nextRunAt?: string;
  lastRunKey?: string;
  lastStartedAt?: string;
  lastFinishedAt?: string;
  lastSuccessAt?: string;
  lastError?: string;
  summary?: string;
  updatedAt?: string;
};

export type ActionCenter = {
  generatedAt: string;
  overall: string;
  items: ActionItem[];
};

export type ActionItem = {
  id: string;
  priority: number;
  level: "success" | "info" | "warning" | "danger" | string;
  title: string;
  description: string;
  count: number;
  target: TabKey;
  filter?: string;
  action: string;
  samples?: string[];
};

export type DetailDrawerKind = "account" | "channel" | "site";

export type DetailDrawerState =
  | { kind: "account"; account: Account }
  | { kind: "channel"; channel: ImportedChannel }
  | { kind: "site"; detail: SiteDetail };

export type SystemSetting = {
  key: string;
  valueJson: string;
  updatedAt: string;
};

export type SystemBackup = {
  fileName: string;
  path: string;
  sizeBytes: number;
  createdAt: string;
};

export type AuditLogItem = {
  id: string;
  action: string;
  level: string;
  actor?: string;
  resourceType?: string;
  resourceId?: string;
  summary: string;
  metadataJson?: string;
  createdAt: string;
};

export type CheckinStatus = {
  generatedAt: string;
  running: boolean;
  mode: string;
  currentAccountId?: string;
  currentAccount?: string;
  currentSite?: string;
  currentMessage?: string;
  totalAccounts: number;
  processedAccounts: number;
  pendingAccounts: number;
  successCount: number;
  alreadyCount: number;
  failedCount: number;
  unsupportedCount: number;
  authExpiredCount: number;
  startedAt?: string;
  updatedAt?: string;
  finishedAt?: string;
  lastRunMessage?: string;
  today: {
    totalLogs: number;
    successCount: number;
    alreadyCount: number;
    failedCount: number;
    unsupportedCount: number;
    authExpiredCount: number;
    dueAccounts: number;
  };
  schedule: {
    enabled: boolean;
    time: string;
    randomDelayMin: number;
    randomDelayMax: number;
    nextRunAt?: string;
    nextWindowStartAt?: string;
    nextWindowEndAt?: string;
    nextRunInSeconds: number;
    nextWindowInSeconds: number;
    message?: string;
  };
};

export type NetworkProxyConfig = {
  enabled: boolean;
  url: string;
  bypassLocal: boolean;
};

export type SyncScheduleConfig = {
  enabled: boolean;
  intervalMinutes: number;
  mode: string;
  runOnStartup: boolean;
};

export type ProxyTestResult = {
  ok: boolean;
  httpStatus?: number;
  latencyMs: number;
  message: string;
  targetUrl: string;
  proxy: NetworkProxyStatus;
};

export type LocalNewAPIInstance = {
  id: string;
  name: string;
  baseUrl: string;
  detectedFrom?: string;
  status: string;
  databasePath?: string;
  channelCount: number;
  hasSyncToken: boolean;
  syncTokenMasked?: string;
  lastScannedAt?: string;
  syncCapability: string;
};

export type SyncPreviewItem = {
  sourceChannelId: string;
  name: string;
  baseUrl?: string;
  status?: string;
  upstreamKind: string;
  action: "new" | "changed" | "unchanged" | "skipped" | "removed" | string;
  reason: string;
  changedFields?: string[];
};

export type LocalNewAPISyncPreview = {
  instanceId: string;
  instanceName: string;
  source: string;
  total: number;
  newCount: number;
  changedCount: number;
  unchangedCount: number;
  skippedCount: number;
  removedCount: number;
  items: SyncPreviewItem[];
  generatedAt: string;
};

export type SyncRunItem = {
  instanceId: string;
  instanceName: string;
  status: "success" | "failed";
  importedCount?: number;
  sitesCreated?: number;
  sitesMerged?: number;
  detectedCount?: number;
  missingCount?: number;
  sourceCount?: number;
  activeCount?: number;
  message?: string;
};

export type SyncRunSummary = {
  scope: "single" | "all" | "mark-missing";
  title: string;
  level: "success" | "warning" | "danger" | "info";
  generatedAt: string;
  items: SyncRunItem[];
};

export type ImportedChannel = {
  id: string;
  localInstanceId?: string;
  sourceChannelId: string;
  sourceType?: string;
  name: string;
  baseUrl?: string;
  status?: string;
  upstreamKind: string;
  supportsCheckin: boolean;
  supportsBalance: boolean;
  supportsModels: boolean;
  supportsPricing: boolean;
  channelKeyMasked?: string;
  modelCount?: number;
  sampleModels?: string[];
  modelsSource?: string;
  modelsStatus?: string;
  modelsLastSyncedAt?: string;
  modelsMessage?: string;
  sourceSyncStatus?: string;
  sourceMissingAt?: string;
  rawJson?: string;
  detectionJson?: string;
  lastDetectedAt?: string;
};

export type UpstreamSite = {
  id: string;
  channelId?: string;
  name: string;
  homepageUrl?: string;
  baseUrl: string;
  loginUrl?: string;
  kind: string;
  detectionConfidence?: number;
  healthStatus: string;
  supportsCheckin: boolean;
  supportsBalance: boolean;
  supportsModels: boolean;
  supportsPricing?: boolean;
  accountCount: number;
  detectionJson?: string;
  lastHealthCheckAt?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type DetectionInfo = {
  baseUrl: string;
  homepageUrl?: string;
  loginUrl?: string;
  kind: string;
  healthStatus: string;
  detectionConfidence: number;
  supportsCheckin: boolean;
  supportsBalance: boolean;
  supportsModels: boolean;
  supportsPricing: boolean;
  matchedSignals: string[];
};

export type SiteDetail = {
  site: UpstreamSite;
  detection: DetectionInfo;
  accounts: Account[];
  balanceSnapshots: BalanceSnapshot[];
  checkinLogs: CheckinLog[];
  suggestions: string[];
};

export type Account = {
  id: string;
  upstreamSiteId: string;
  upstreamSiteName: string;
  upstreamSiteBaseUrl?: string;
  upstreamSiteLoginUrl?: string;
  upstreamSiteKind?: string;
  displayName: string;
  email?: string;
  username?: string;
  authType: string;
  loginStatus: string;
  apiKeyFingerprint?: string;
  apiKeyStatus?: string;
  apiKeyLastCheckedAt?: string;
  apiKeyModelCount?: number;
  apiKeySampleModels?: string[];
  apiKeyTestModel?: string;
  apiKeyModelUsable?: boolean;
  apiKeyLatencyMs?: number;
  apiKeyTestHttpStatus?: number;
  apiKeyTestMessage?: string;
  apiKeyTestPath?: string;
  balance?: number;
  balanceUnit?: string;
  lastCheckinAt?: string;
  lastCheckinStatus?: string;
  lastCheckinMessage?: string;
  browserProfilePath?: string;
  lastLoginAt?: string;
  lastValidatedAt?: string;
  cookieExpiryAt?: string;
  storageStateExpiryAt?: string;
};

export type UnsupportedCheckinAccountItem = {
  accountId: string;
  accountName: string;
  upstreamSiteId: string;
  upstreamSiteName: string;
  upstreamSiteKind: string;
  lastCheckinStatus?: string;
  reason: string;
};

export type UnsupportedCheckinCleanupResult = {
  matched: number;
  deleted: number;
  limit: number;
  hasMore: boolean;
  dryRun: boolean;
  includeLastUnsupported: boolean;
  items: UnsupportedCheckinAccountItem[];
};
export type CheckinLog = {
  id: string;
  accountName: string;
  upstreamSiteName: string;
  status: string;
  message?: string;
  reward?: string;
  startedAt: string;
};

export type BalanceSnapshot = {
  id: string;
  accountName: string;
  upstreamSiteName: string;
  balance?: number;
  usedQuota?: number;
  totalQuota?: number;
  unit: string;
  createdAt: string;
};

export type UsageAccountItem = {
  accountId: string;
  accountName: string;
  siteId: string;
  siteName: string;
  balance?: number;
  previousBalance?: number;
  balanceDelta?: number;
  unit: string;
  estimatedDailyUse?: number;
  lowBalance: boolean;
  trend: string;
  lastSnapshotAt?: string;
  previousSnapshotAt?: string;
};

export type UsageSiteItem = {
  siteId: string;
  siteName: string;
  accountCount: number;
  lowBalanceCount: number;
  decliningCount: number;
  balanceByUnit: Record<string, number>;
  estimatedDailyUse: Record<string, number>;
};

export type UsageOverview = {
  generatedAt: string;
  accountCount: number;
  siteCount: number;
  lowBalanceCount: number;
  decliningCount: number;
  estimatedDailyUse: Record<string, number>;
  sites: UsageSiteItem[];
  accounts: UsageAccountItem[];
};

export type NotificationItem = {
  id: string;
  type: string;
  level: string;
  title: string;
  content: string;
  read: boolean;
  createdAt: string;
};

export type ChromePasswordMatch = {
  siteId: string;
  siteName: string;
  siteBaseUrl: string;
  chromeName: string;
  url: string;
  username: string;
  passwordMasked: string;
  existingAccount: boolean;
};

export type ChromePasswordPreview = {
  totalRows: number;
  matchedRows: number;
  uniqueSiteCount: number;
  matches: ChromePasswordMatch[];
};

export type BulkPasswordLoginResponse = {
  processed: number;
  success: number;
  failed: number;
};

export type BulkAPIKeyTestResponse = {
  processed: number;
  valid: number;
  invalid: number;
  usable?: number;
};

export type APIKeyTestResult = {
  accountId: string;
  accountName?: string;
  siteName?: string;
  fingerprint?: string;
  status: string;
  httpStatus?: number;
  path?: string;
  message?: string;
  modelCount?: number;
  sampleModels?: string[];
  testedModel?: string;
  modelUsable: boolean;
  modelTestHttpStatus?: number;
  modelTestLatencyMs?: number;
  modelTestMessage?: string;
  modelTestPath?: string;
};

export type ModelCoverageOverviewItem = {
  model: string;
  accountCount: number;
  validKeyCount: number;
  usableCount: number;
  fastestLatencyMs?: number;
  sites?: string[];
  fingerprints?: string[];
};

export type SiteModelCoverageOverviewItem = {
  siteId: string;
  siteName: string;
  baseUrl: string;
  kind: string;
  modelCount: number;
  validKeyCount: number;
  usableKeyCount: number;
  fastestLatencyMs?: number;
  sampleModels?: string[];
};

export type ModelPriceHint = {
  model: string;
  vendor: string;
  priceLevel: string;
  notes: string;
};

export type ModelOverview = {
  generatedAt: string;
  syncedAccounts?: number;
  modelCount: number;
  accountCount: number;
  validKeyCount: number;
  usableModelCount: number;
  fastestLatencyMs?: number;
  models: ModelCoverageOverviewItem[];
  sites: SiteModelCoverageOverviewItem[];
  priceHints: ModelPriceHint[];
};

export type ModelPricingSource = {
  channelId: string;
  channelName: string;
  baseUrl?: string;
  kind: string;
  model: string;
  upstreamModel?: string;
  source: string;
  fieldPath: string;
  price?: number;
  promptRatio?: number;
  completionRatio?: number;
  unit?: string;
  currency?: string;
  confidence: string;
  notes?: string;
  rawValueMasked?: string;
};

export type SitePricingCacheItem = {
  siteId: string;
  siteName: string;
  baseUrl: string;
  kind: string;
  status: string;
  httpStatus?: number;
  latencyMs?: number;
  sourcePath: string;
  sourceCount: number;
  modelCount: number;
  message?: string;
  lastSyncedAt?: string;
};

export type ModelPriceComparison = {
  model: string;
  sourceCount: number;
  siteCount: number;
  usableAccountCount: number;
  fastestLatencyMs?: number;
  lowestPrice?: number;
  lowestPromptRatio?: number;
  lowestCompletionRatio?: number;
  bestSource?: string;
  sites?: string[];
  notes?: string;
};

export type ModelPricingOverview = {
  generatedAt: string;
  sourceCount: number;
  modelCount: number;
  exactCount: number;
  ratioCount: number;
  liveCacheCount?: number;
  failedCacheCount?: number;
  sources: ModelPricingSource[];
  siteCaches?: SitePricingCacheItem[];
  comparisons?: ModelPriceComparison[];
};

export type ChannelModelSyncItem = {
  channelId: string;
  channelName: string;
  baseUrl?: string;
  kind: string;
  hasKey: boolean;
  status: string;
  source?: string;
  modelCount: number;
  sampleModels?: string[];
  latencyMs?: number;
  message?: string;
  lastSyncedAt?: string;
};

export type ChannelModelCoverageItem = {
  model: string;
  channelCount: number;
  liveKeyCount: number;
  channels?: string[];
};

export type ChannelModelOverview = {
  generatedAt: string;
  syncedChannels?: number;
  channelCount: number;
  modelCount: number;
  liveKeyCount: number;
  rawOnlyCount: number;
  failedCount: number;
  uncheckedCount: number;
  items: ChannelModelSyncItem[];
  models: ChannelModelCoverageItem[];
};

export type KeyExportPreviewItem = {
  accountId: string;
  accountName: string;
  siteName: string;
  baseUrl: string;
  fingerprint: string;
  status: string;
  modelCount: number;
  sampleModels?: string[];
  testModel?: string;
  modelUsable: boolean;
  latencyMs?: number;
  lastCheckedAt?: string;
  maskedExportRef: string;
};

export type KeyExportPreview = {
  generatedAt: string;
  total: number;
  valid: number;
  usable: number;
  items: KeyExportPreviewItem[];
  notice: string;
};

export type BulkBrowserOpenResponse = {
  processed: number;
  opened: number;
  failed: number;
};

export type BulkBrowserSaveResponse = {
  processed: number;
  saved: number;
  failed: number;
};

export type BulkDetectSitesResponse = {
  processed: number;
  identified: number;
};

export type BulkBalanceRefreshResponse = {
  processed: number;
  success: number;
  failed: number;
};

export type ActionQuickAction = {
  id: string;
  label: string;
  variant?: "primary" | "ghost" | "danger";
};

export type ApiResult<T> = {
  ok: boolean;
  data?: T;
  error?: string;
  errorClass?: string;
};

export type SessionPayload = {
  authenticated: boolean;
  userId?: string;
};

export type ClientReadCacheEntry = {
  expiresAt: number;
  promise: Promise<unknown>;
};

export type GlobalApiError = {
  message: string;
  errorClass?: string;
  status?: number;
  url: string;
  method: string;
  occurredAt: number;
};

export type NavigationIntent = {
  target: TabKey;
  sourceStatus?: string;
  channelKind?: string;
  accountStatus?: string;
  checkinStatus?: string;
  siteHealth?: string;
  siteKind?: string;
  unreadOnly?: boolean;
  query?: string;
};
