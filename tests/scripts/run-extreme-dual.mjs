#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import { spawnSync } from "node:child_process";
import Browserbase from "@browserbasehq/sdk";

const rootDir = process.cwd();
const resultsRoot = path.join(rootDir, "test-results", "extreme");
const runId = new Date().toISOString().replace(/[:.]/g, "-");
const runDir = path.join(resultsRoot, runId);
fs.mkdirSync(runDir, { recursive: true });

function runPlaywright(target, env = {}) {
  const outputFile = path.join(runDir, `raw-${target}.json`);
  const cmdEnv = {
    ...process.env,
    ...env,
    PW_JSON_REPORT_PATH: outputFile,
  };

  const result = spawnSync("npx", ["playwright", "test"], {
    cwd: rootDir,
    env: cmdEnv,
    stdio: "inherit",
    shell: process.platform === "win32",
  });

  const code = result.status ?? 1;
  return { code, outputFile };
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function parseReport(reportPath) {
  if (!fs.existsSync(reportPath)) {
    return {
      reportPath,
      totals: { passed: 0, failed: 0, skipped: 0, timedOut: 0, interrupted: 0 },
      failures: [{ title: "Missing report output", message: reportPath }],
    };
  }

  const report = JSON.parse(fs.readFileSync(reportPath, "utf8"));
  const totals = { passed: 0, failed: 0, skipped: 0, timedOut: 0, interrupted: 0 };
  const failures = [];

  const classifyFailure = (title, message) => {
    const haystack = `${title}\n${message}`.toLowerCase();
    if (
      haystack.includes("429") ||
      haystack.includes("too many requests") ||
      haystack.includes("browsertype.connect") ||
      haystack.includes("could not find a running session") ||
      haystack.includes("max concurrent sessions")
    ) {
      return "blocked_infra";
    }
    if (haystack.includes("check your email") || haystack.includes("strict mode violation")) {
      return "test_contract_drift";
    }
    if (haystack.includes("retry #1") || haystack.includes("flaky")) {
      return "flaky";
    }
    return "product_defect";
  };

  const inferModule = (title) => {
    const t = title.toLowerCase();
    if (t.includes("auth") || t.includes("login")) return "auth";
    if (t.includes("signup") || t.includes("register")) return "signup";
    if (t.includes("client")) return "clients";
    if (t.includes("membership")) return "memberships";
    if (t.includes("payment") || t.includes("gocardless") || t.includes("stripe")) return "payments-integrations";
    if (t.includes("report") || t.includes("dashboard")) return "reports-dashboard";
    return "general";
  };

  const walkSuite = (suite, parentTitle = "") => {
    for (const spec of suite.specs || []) {
      for (const test of spec.tests || []) {
        const results = test.results || [];
        const finalResult = results[results.length - 1] || {};
        const status = finalResult.status || "interrupted";
        if (status in totals) totals[status] += 1;
        else totals.interrupted += 1;

        if (status === "failed" || status === "timedOut") {
          const message =
            finalResult.error?.message ||
            finalResult.errors?.[0]?.message ||
            "No error message captured";
          const fullTitle = [parentTitle, spec.title, test.title].filter(Boolean).join(" > ");
          const endpointMatch = message.match(/\/api\/[a-zA-Z0-9/_-]+/);
          const failedAttempts = results.filter((r) => r.status === "failed" || r.status === "timedOut").length;
          const reproducibility = results.length === 0 ? 1 : Number((failedAttempts / results.length).toFixed(2));
          failures.push({
            title: fullTitle,
            message,
            classification: classifyFailure(fullTitle, message),
            module: inferModule(fullTitle),
            endpoint: endpointMatch ? endpointMatch[0] : null,
            reproducibility,
          });
        }
      }
    }
    for (const child of suite.suites || []) {
      walkSuite(child, [parentTitle, child.title].filter(Boolean).join(" > "));
    }
  };

  for (const suite of report.suites || []) {
    walkSuite(suite, suite.title || "");
  }

  return { reportPath, totals, failures };
}

function makeMarkdown(summary) {
  const lines = [];
  lines.push("# Extreme Test Report");
  lines.push("");
  lines.push(`- Run ID: \`${summary.runId}\``);
  lines.push(`- Started: \`${summary.startedAt}\``);
  lines.push(`- Finished: \`${summary.finishedAt}\``);
  lines.push("");
  lines.push("## Results");
  lines.push("");
  for (const run of summary.runs) {
    const t = run.totals;
    lines.push(`### ${run.target}`);
    lines.push(`- Exit code: \`${run.exitCode}\``);
    if (run.browserbaseSessionId) {
      lines.push(`- Browserbase session: \`${run.browserbaseSessionId}\``);
      lines.push(`- Replay: https://browserbase.com/sessions/${run.browserbaseSessionId}`);
    }
    lines.push(`- Passed: ${t.passed}`);
    lines.push(`- Failed: ${t.failed}`);
    lines.push(`- Skipped: ${t.skipped}`);
    lines.push(`- Timed out: ${t.timedOut}`);
    lines.push(`- Interrupted: ${t.interrupted}`);
    lines.push("");

    if (run.failures.length > 0) {
      lines.push("#### Failures");
      for (const fail of run.failures) {
        lines.push(`- [${fail.classification}] ${fail.title} (module: ${fail.module}, repro: ${fail.reproducibility})`);
      }
      lines.push("");
    }
  }

  return `${lines.join("\n")}\n`;
}

async function releaseRunningSessions(bb, projectId) {
  const sessions = await bb.sessions.list({ status: "RUNNING" });
  const candidates = (sessions || []).filter((s) => !projectId || s.projectId === projectId);
  for (const session of candidates) {
    try {
      await bb.sessions.update(session.id, { status: "REQUEST_RELEASE", projectId });
      // Keep output visible in CI/local logs for debug.
      console.log(`Released Browserbase session: ${session.id}`);
    } catch (err) {
      console.warn(`Failed to release Browserbase session ${session.id}: ${err?.message || err}`);
    }
  }
}

function buildBlockedRun(target, message) {
  return {
    target,
    exitCode: 1,
    reportPath: "",
    totals: { passed: 0, failed: 0, skipped: 0, timedOut: 0, interrupted: 0 },
    failures: [{ title: `${target} blocked`, message }],
    blockedReason: message,
  };
}

const startedAt = new Date().toISOString();
const runs = [];

// 1) Local run
const localRun = runPlaywright("local");
const localParsed = parseReport(localRun.outputFile);
runs.push({
  target: "playwright-local",
  exitCode: localRun.code,
  ...localParsed,
});

// 2) Browserbase run
const apiKey = process.env.BROWSERBASE_API_KEY;
const projectId = process.env.BROWSERBASE_PROJECT_ID;
if (!apiKey || !projectId) {
  console.error("BROWSERBASE_API_KEY and BROWSERBASE_PROJECT_ID are required for Browserbase run.");
  process.exit(1);
}

const bb = new Browserbase({ apiKey });
const browserbaseTimeoutSeconds = Number(process.env.BROWSERBASE_SESSION_TIMEOUT_SECONDS || 7200);
const browserbaseConnectTimeout = process.env.BROWSERBASE_CONNECT_TIMEOUT_MS || "15000";
let browserbaseSession = null;
try {
  await releaseRunningSessions(bb, projectId);
  await sleep(2000);

  // Retry once in case account-level concurrency/rate-limit is briefly stale.
  let createErr = null;
  for (let attempt = 0; attempt < 2; attempt += 1) {
    try {
      browserbaseSession = await bb.sessions.create({
        projectId,
        keepAlive: true,
        timeout: Number.isFinite(browserbaseTimeoutSeconds) ? browserbaseTimeoutSeconds : 7200,
      });
      break;
    } catch (err) {
      createErr = err;
      await releaseRunningSessions(bb, projectId);
      await sleep(3000);
    }
  }

  if (!browserbaseSession) {
    throw createErr || new Error("failed to create Browserbase session");
  }

  const browserbaseRun = runPlaywright("browserbase", {
    BROWSERBASE_CONNECT_URL: browserbaseSession.connectUrl,
    BROWSERBASE_CONNECT_TIMEOUT_MS: browserbaseConnectTimeout,
  });
  const browserbaseParsed = parseReport(browserbaseRun.outputFile);
  runs.push({
    target: "playwright-browserbase",
    exitCode: browserbaseRun.code,
    browserbaseSessionId: browserbaseSession.id,
    ...browserbaseParsed,
  });
} catch (err) {
  const message = err?.message || String(err);
  runs.push({
    ...buildBlockedRun("playwright-browserbase", message),
    browserbaseSessionId: browserbaseSession?.id,
  });
} finally {
  if (browserbaseSession?.id) {
    try {
      await bb.sessions.update(browserbaseSession.id, { status: "REQUEST_RELEASE", projectId });
    } catch {
      // no-op
    }
  }
}

const finishedAt = new Date().toISOString();
const summary = { runId, startedAt, finishedAt, runs };

const summaryJsonPath = path.join(runDir, "run-summary.json");
const summaryMdPath = path.join(runDir, "run-summary.md");
fs.writeFileSync(summaryJsonPath, JSON.stringify(summary, null, 2));
fs.writeFileSync(summaryMdPath, makeMarkdown(summary));

console.log(`\nCombined report written to:\n- ${summaryJsonPath}\n- ${summaryMdPath}`);

const failed = runs.some((r) => r.exitCode !== 0 || r.totals.failed > 0 || r.totals.timedOut > 0);
process.exit(failed ? 1 : 0);
