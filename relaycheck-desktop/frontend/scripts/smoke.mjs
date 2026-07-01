#!/usr/bin/env node
// smoke.mjs — thin wrapper that delegates to verify-navigation.mjs
// Usage: npm run smoke
// Prerequisites:
//   1. Dev server running (http://127.0.0.1:5173)
//   2. playwright installed (npx playwright install chromium)
//   3. RELAYCHECK_SMOKE_PASSWORD set if the app requires auth

import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const verifier = join(__dirname, "verify-navigation.mjs");

try {
  await import(verifier);
} catch (err) {
  if (err.code === "ERR_MODULE_NOT_FOUND" || err.code === "MODULE_NOT_FOUND") {
    // playwright may not be installed — give a helpful message
    console.error("Failed to load verify-navigation.mjs.");
    console.error("Prerequisites:");
    console.error("  1. Dev server: cd frontend && npm run dev");
    console.error("  2. Playwright:  npx playwright install chromium");
    console.error("  3. (Optional)   RELAYCHECK_SMOKE_PASSWORD=<pwd>");
    process.exit(1);
  }
  throw err;
}
