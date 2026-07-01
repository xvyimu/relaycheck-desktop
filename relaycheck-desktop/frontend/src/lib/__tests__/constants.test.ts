import { describe, it, expect } from "vitest";
import {
  NAV_ITEMS,
  STATUS_ICON_SUCCESS_LEVELS,
  STATUS_ICON_WARNING_LEVELS,
  STATUS_ICON_DANGER_LEVELS,
  TARGET_RELAY_KINDS,
  PROBLEM_LOGIN_STATUSES,
  PROBLEM_CHECKIN_STATUSES,
  SUCCESSFUL_CHECKIN_STATUSES,
  CHANNEL_RAW_SEARCH_KEYS,
  CHANNELS_INITIAL_VISIBLE_LIMIT,
  CHANNELS_VISIBLE_INCREMENT,
} from "../constants";

describe("NAV_ITEMS", () => {
  it("is a non-empty array", () => {
    expect(NAV_ITEMS.length).toBeGreaterThan(0);
  });

  it("each item has required keys", () => {
    for (const item of NAV_ITEMS) {
      expect(item).toHaveProperty("key");
      expect(item).toHaveProperty("label");
      expect(item).toHaveProperty("icon");
      expect(item).toHaveProperty("description");
    }
  });

  it("keys are unique", () => {
    const keys = NAV_ITEMS.map((item) => item.key);
    expect(new Set(keys).size).toBe(keys.length);
  });

  it("contains expected navigation keys", () => {
    const keys = NAV_ITEMS.map((item) => item.key);
    expect(keys).toContain("dashboard");
    expect(keys).toContain("channels");
    expect(keys).toContain("settings");
  });
});

describe("STATUS_ICON_SUCCESS_LEVELS", () => {
  it("contains expected success levels", () => {
    expect(STATUS_ICON_SUCCESS_LEVELS.has("success")).toBe(true);
    expect(STATUS_ICON_SUCCESS_LEVELS.has("valid")).toBe(true);
    expect(STATUS_ICON_SUCCESS_LEVELS.has("active")).toBe(true);
    expect(STATUS_ICON_SUCCESS_LEVELS.has("enabled")).toBe(true);
    expect(STATUS_ICON_SUCCESS_LEVELS.has("ok")).toBe(true);
  });

  it("does not contain warning or danger levels", () => {
    expect(STATUS_ICON_SUCCESS_LEVELS.has("error")).toBe(false);
    expect(STATUS_ICON_SUCCESS_LEVELS.has("warning")).toBe(false);
    expect(STATUS_ICON_SUCCESS_LEVELS.has("danger")).toBe(false);
  });
});

describe("STATUS_ICON_WARNING_LEVELS", () => {
  it("contains expected warning levels", () => {
    expect(STATUS_ICON_WARNING_LEVELS.has("warning")).toBe(true);
    expect(STATUS_ICON_WARNING_LEVELS.has("missing")).toBe(true);
    expect(STATUS_ICON_WARNING_LEVELS.has("archived")).toBe(true);
    expect(STATUS_ICON_WARNING_LEVELS.has("idle")).toBe(true);
  });

  it("does not contain success or danger levels", () => {
    expect(STATUS_ICON_WARNING_LEVELS.has("success")).toBe(false);
    expect(STATUS_ICON_WARNING_LEVELS.has("error")).toBe(false);
  });
});

describe("STATUS_ICON_DANGER_LEVELS", () => {
  it("contains expected danger levels", () => {
    expect(STATUS_ICON_DANGER_LEVELS.has("error")).toBe(true);
    expect(STATUS_ICON_DANGER_LEVELS.has("failed")).toBe(true);
    expect(STATUS_ICON_DANGER_LEVELS.has("invalid")).toBe(true);
    expect(STATUS_ICON_DANGER_LEVELS.has("expired")).toBe(true);
    expect(STATUS_ICON_DANGER_LEVELS.has("disabled")).toBe(true);
  });
});

describe("TARGET_RELAY_KINDS", () => {
  it("contains newapi and oneapi", () => {
    expect(TARGET_RELAY_KINDS.has("newapi")).toBe(true);
    expect(TARGET_RELAY_KINDS.has("oneapi")).toBe(true);
    expect(TARGET_RELAY_KINDS.has("sub2api")).toBe(true);
    expect(TARGET_RELAY_KINDS.has("modified_relay")).toBe(true);
  });

  it("does not contain unknown", () => {
    expect(TARGET_RELAY_KINDS.has("unknown")).toBe(false);
  });
});

describe("PROBLEM_LOGIN_STATUSES", () => {
  it("contains expected login problem statuses", () => {
    expect(PROBLEM_LOGIN_STATUSES.has("expired")).toBe(true);
    expect(PROBLEM_LOGIN_STATUSES.has("manual_required")).toBe(true);
    expect(PROBLEM_LOGIN_STATUSES.has("captcha_required")).toBe(true);
    expect(PROBLEM_LOGIN_STATUSES.has("two_factor_required")).toBe(true);
  });

  it("does not contain valid status", () => {
    expect(PROBLEM_LOGIN_STATUSES.has("valid")).toBe(false);
  });
});

describe("PROBLEM_CHECKIN_STATUSES", () => {
  it("contains expected checkin problem statuses", () => {
    expect(PROBLEM_CHECKIN_STATUSES.has("auth_expired")).toBe(true);
    expect(PROBLEM_CHECKIN_STATUSES.has("manual_required")).toBe(true);
    expect(PROBLEM_CHECKIN_STATUSES.has("failed")).toBe(true);
  });
});

describe("SUCCESSFUL_CHECKIN_STATUSES", () => {
  it("contains success and already_checked", () => {
    expect(SUCCESSFUL_CHECKIN_STATUSES.has("success")).toBe(true);
    expect(SUCCESSFUL_CHECKIN_STATUSES.has("already_checked")).toBe(true);
  });
});

describe("CHANNEL_RAW_SEARCH_KEYS", () => {
  it("contains name and type", () => {
    expect(CHANNEL_RAW_SEARCH_KEYS.has("name")).toBe(true);
    expect(CHANNEL_RAW_SEARCH_KEYS.has("type")).toBe(true);
  });

  it("does not contain arbitrary keys", () => {
    expect(CHANNEL_RAW_SEARCH_KEYS.has("nonexistent")).toBe(false);
  });
});

describe("pagination constants", () => {
  it("CHANNELS_INITIAL_VISIBLE_LIMIT is a positive number", () => {
    expect(CHANNELS_INITIAL_VISIBLE_LIMIT).toBeGreaterThan(0);
  });

  it("CHANNELS_VISIBLE_INCREMENT is a positive number", () => {
    expect(CHANNELS_VISIBLE_INCREMENT).toBeGreaterThan(0);
  });

  it("default pagination values are 24", () => {
    expect(CHANNELS_INITIAL_VISIBLE_LIMIT).toBe(24);
    expect(CHANNELS_VISIBLE_INCREMENT).toBe(24);
  });
});
