import { useEffect, useMemo, useState } from "react";
import {
  CHANNELS_INITIAL_VISIBLE_LIMIT,
  CHANNELS_VISIBLE_INCREMENT,
  CHANNEL_RAW_SEARCH_KEYS,
  TARGET_RELAY_KINDS,
} from "@/lib/constants";
import type { Account, ImportedChannel, NavigationIntent } from "@/types";

function isTargetRelayKindUI(kind?: string | null): boolean {
  return TARGET_RELAY_KINDS.has(kind || "");
}

function safeParseJSON(value: string): unknown | null {
  if (!value.trim()) return null;
  try {
    return JSON.parse(value);
  } catch {
    return null;
  }
}

function normalizeSearchURL(value: string): string {
  return value.replace(/^https?:\/\//, "").replace(/\/+$/, "").toLowerCase();
}

function rawChannelSearchText(rawJson?: string): string {
  const parsed = safeParseJSON(rawJson || "");
  const parts: string[] = [];

  function visit(value: unknown, key = "", depth = 0) {
    if (depth > 4 || value === null || value === undefined) return;
    const normalizedKey = key.replace(/[_-]/g, "").toLowerCase();
    const shouldCollect =
      CHANNEL_RAW_SEARCH_KEYS.has(key.toLowerCase()) || CHANNEL_RAW_SEARCH_KEYS.has(normalizedKey);

    if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
      if (shouldCollect) parts.push(String(value));
      return;
    }

    if (Array.isArray(value)) {
      value.forEach((item) => {
        if (shouldCollect && (typeof item === "string" || typeof item === "number" || typeof item === "boolean")) {
          parts.push(String(item));
          return;
        }
        visit(item, key, depth + 1);
      });
      return;
    }

    if (typeof value === "object") {
      Object.entries(value as Record<string, unknown>).forEach(([childKey, childValue]) => {
        visit(childValue, childKey, depth + 1);
      });
    }
  }

  visit(parsed);
  return parts.join(" ");
}

function channelAccountSearchText(channel: ImportedChannel, accounts: Account[]): string {
  const channelBase = normalizeSearchURL(channel.baseUrl || "");
  const channelName = channel.name.trim().toLowerCase();
  return accounts
    .filter((account) => {
      const accountBase = normalizeSearchURL(account.upstreamSiteBaseUrl || "");
      const accountSite = account.upstreamSiteName.trim().toLowerCase();
      return (
        (channelBase && accountBase && channelBase === accountBase) ||
        (channelName && accountSite && (channelName.includes(accountSite) || accountSite.includes(channelName)))
      );
    })
    .flatMap((account) => [account.displayName, account.email || "", account.username || ""])
    .join(" ")
    .toLowerCase();
}

export interface ChannelFiltersResult {
  query: string;
  setQuery: (value: string) => void;
  sourceStatusFilter: string;
  setSourceStatusFilter: (value: string) => void;
  kindFilter: string;
  setKindFilter: (value: string) => void;
  kindOptions: string[];
  visibleChannels: ImportedChannel[];
  displayedChannels: ImportedChannel[];
  hasMoreChannels: boolean;
  visibleLimit: number;
  setVisibleLimit: (value: number | ((prev: number) => number)) => void;
  identifiedCount: number;
  checkinCount: number;
  targetRelayCount: number;
  missingBaseUrlCount: number;
  sourceMissingCount: number;
  sourceArchivedCount: number;
  clearFilters: () => void;
}

export function useChannelFilters(
  channels: ImportedChannel[],
  accounts: Account[],
  intent?: NavigationIntent | null,
): ChannelFiltersResult {
  const [query, setQuery] = useState("");
  const [sourceStatusFilter, setSourceStatusFilter] = useState("not_archived");
  const [kindFilter, setKindFilter] = useState("target_relay");
  const [visibleLimit, setVisibleLimit] = useState(CHANNELS_INITIAL_VISIBLE_LIMIT);

  const identifiedCount = channels.filter(
    (channel) => channel.upstreamKind && channel.upstreamKind !== "unknown",
  ).length;
  const checkinCount = channels.filter((channel) => channel.supportsCheckin).length;
  const targetRelayCount = channels.filter((channel) =>
    isTargetRelayKindUI(channel.upstreamKind),
  ).length;
  const missingBaseUrlCount = channels.filter((channel) => !channel.baseUrl).length;
  const sourceMissingCount = channels.filter(
    (channel) => channel.sourceSyncStatus === "missing",
  ).length;
  const sourceArchivedCount = channels.filter(
    (channel) => channel.sourceSyncStatus === "archived",
  ).length;

  const kindOptions = useMemo(() => {
    return Array.from(new Set(channels.map((channel) => channel.upstreamKind || "unknown"))).sort();
  }, [channels]);

  const visibleChannels = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    return channels.filter((channel) => {
      const sourceStatus = channel.sourceSyncStatus || "active";
      if (sourceStatusFilter === "not_archived" && sourceStatus === "archived") return false;
      if (sourceStatusFilter !== "all" && sourceStatusFilter !== "not_archived" && sourceStatus !== sourceStatusFilter) return false;
      if (kindFilter === "target_relay" && !isTargetRelayKindUI(channel.upstreamKind)) return false;
      if (kindFilter !== "all" && kindFilter !== "target_relay" && (channel.upstreamKind || "unknown") !== kindFilter) return false;
      if (!normalizedQuery) return true;
      const combined = [
        channel.name,
        channel.baseUrl || "",
        channel.sourceChannelId,
        channel.status || "",
        channel.upstreamKind || "",
        channel.sourceType || "",
        channel.modelsMessage || "",
        channelAccountSearchText(channel, accounts),
        rawChannelSearchText(channel.rawJson),
      ].join(" ").toLowerCase();
      return combined.includes(normalizedQuery);
    });
  }, [accounts, channels, kindFilter, query, sourceStatusFilter]);

  const displayedChannels = visibleChannels.slice(0, visibleLimit);
  const hasMoreChannels = visibleChannels.length > displayedChannels.length;

  useEffect(() => {
    if (!intent) return;
    if (intent.sourceStatus) setSourceStatusFilter(intent.sourceStatus);
    if (intent.channelKind) setKindFilter(intent.channelKind === "unknown" ? "unknown" : intent.channelKind);
    if (typeof intent.query === "string") setQuery(intent.query);
  }, [intent]);

  useEffect(() => {
    setVisibleLimit(CHANNELS_INITIAL_VISIBLE_LIMIT);
  }, [query, sourceStatusFilter, kindFilter]);

  function clearFilters() {
    setQuery("");
    setSourceStatusFilter("not_archived");
    setKindFilter("target_relay");
  }

  return {
    query, setQuery,
    sourceStatusFilter, setSourceStatusFilter,
    kindFilter, setKindFilter,
    kindOptions,
    visibleChannels,
    displayedChannels,
    hasMoreChannels,
    visibleLimit, setVisibleLimit,
    identifiedCount,
    checkinCount,
    targetRelayCount,
    missingBaseUrlCount,
    sourceMissingCount,
    sourceArchivedCount,
    clearFilters,
  };
}