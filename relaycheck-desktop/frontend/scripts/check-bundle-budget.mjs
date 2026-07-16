#!/usr/bin/env node
import { createGzip } from "node:zlib";
import { createReadStream, readdirSync, statSync, writeFileSync } from "node:fs";
import { join, basename } from "node:path";
import { pipeline } from "node:stream/promises";
import { Writable } from "node:stream";

const distDir = process.argv[2] || "dist/assets";
const budgets = {
  mainJsGzip: 80 * 1024,
  mainCssGzip: 40 * 1024,
  panelGzip: 45 * 1024,
};

function gzipSize(file) {
  return new Promise((resolve, reject) => {
    let size = 0;
    const sink = new Writable({
      write(chunk, _enc, cb) {
        size += chunk.length;
        cb();
      },
    });
    pipeline(createReadStream(file), createGzip(), sink)
      .then(() => resolve(size))
      .catch(reject);
  });
}

const files = readdirSync(distDir)
  .filter((name) => name.endsWith(".js") || name.endsWith(".css"))
  .map((name) => join(distDir, name));

const rows = [];
let failed = false;
for (const file of files) {
  const gz = await gzipSize(file);
  const name = basename(file);
  let budget = null;
  let kind = "other";
  if (/^index-.*\.js$/.test(name)) {
    kind = "main-js";
    budget = budgets.mainJsGzip;
  } else if (/^index-.*\.css$/.test(name)) {
    kind = "main-css";
    budget = budgets.mainCssGzip;
  } else if (/^panel-.*\.js$/.test(name)) {
    kind = "panel";
    budget = budgets.panelGzip;
  }
  const raw = statSync(file).size;
  const over = budget != null && gz > budget;
  if (over) failed = true;
  rows.push({ name, kind, raw, gzip: gz, budget, over });
  const budgetLabel = budget == null ? "-" : `${(budget / 1024).toFixed(1)} kB`;
  console.log(
    `${over ? "FAIL" : "ok  "} ${name.padEnd(42)} raw=${(raw / 1024).toFixed(1)}kB gzip=${(gz / 1024).toFixed(1)}kB budget=${budgetLabel}`,
  );
}

writeFileSync(join(distDir, "..", "bundle-budget.json"), JSON.stringify({ budgets, rows }, null, 2));
if (failed) {
  console.error("Bundle budget exceeded.");
  process.exit(1);
}
console.log("Bundle budget OK.");
