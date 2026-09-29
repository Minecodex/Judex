// Each acceptance run owns its server, PostgreSQL and S3 containers.
// Existing developer services and previous test containers are never reused.
import { execFileSync, spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import net from "node:net";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const root = fileURLToPath(new URL("../../", import.meta.url));
const runId = `judex-e2e-${process.pid}-${Date.now()}`;
const pg = `${runId}-pg`, s3 = `${runId}-s3`;
const artifact = path.join(root, ".cache/e2e", runId);
fs.mkdirSync(artifact, { recursive: true });
const password = randomBytes(18).toString("hex");
const run = (command, args, options = {}) => String(execFileSync(command, args, { cwd: root, encoding: "utf8", windowsHide: true, ...options }) ?? "").trim();
const docker = (...args) => run("docker", args);
const npm = process.env.npm_execpath || path.join(path.dirname(process.execPath), "node_modules/npm/bin/npm-cli.js");
let server;
async function waitFor(url) {
  const deadline = Date.now() + 60000;
  while (Date.now() < deadline) {
    if (server?.exitCode != null) throw new Error("Acceptance server exited; inspect server.log");
    try { if ((await fetch(url)).ok) return; } catch {}
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error(`Readiness timed out: ${url}`);
}
try {
  run(process.execPath, [npm, "run", "build"], { stdio: "inherit" });
  const executable = path.join(artifact, process.platform === "win32" ? "judex-server.exe" : "judex-server");
  run("go", ["build", "-o", executable, "./cmd/judex-server"]);
  const cli = path.join(artifact, process.platform === "win32" ? "judex.exe" : "judex");
  run("go", ["build", "-o", cli, "./cmd/judex"]);
  docker("run", "-d", "--rm", "--name", pg, "--label", `judex.test-run=${runId}`, "-e", `POSTGRES_PASSWORD=${password}`, "-e", "POSTGRES_DB=judex", "-p", "127.0.0.1::5432", "postgres:17-alpine");
  docker("run", "-d", "--rm", "--name", s3, "--label", `judex.test-run=${runId}`, "-e", "MINIO_ROOT_USER=judex", "-e", `MINIO_ROOT_PASSWORD=${password}`, "-p", "127.0.0.1::9000", "quay.io/minio/minio:RELEASE.2025-06-13T11-33-47Z", "server", "/data");
  const pgDeadline=Date.now()+60000;
  for (;;) {
    try { docker("exec",pg,"pg_isready","-h","127.0.0.1","-U","postgres","-d","judex"); break; }
    catch { if(Date.now()>pgDeadline) throw new Error("PostgreSQL readiness timed out"); await new Promise((resolve)=>setTimeout(resolve,250)); }
  }
  const pgPort = docker("port", pg, "5432").split(":").at(-1);
  const s3Port = docker("port", s3, "9000").split(":").at(-1);
  await waitFor(`http://127.0.0.1:${s3Port}/minio/health/ready`);
  const listener = net.createServer(); await new Promise((resolve) => listener.listen(0, "127.0.0.1", resolve));
  const port = listener.address().port; await new Promise((resolve) => listener.close(resolve));
  const base = `http://127.0.0.1:${port}`;
  const env = { ...process.env, GIN_MODE: "release", JUDEX_ENV: "development", JUDEX_MODE: "all", JUDEX_HTTP_ADDR: `127.0.0.1:${port}`, JUDEX_WEB_DIR: "web/dist",
    JUDEX_DATABASE_URL: `postgres://postgres:${password}@127.0.0.1:${pgPort}/judex?sslmode=disable`, JUDEX_ALLOWED_ORIGINS: base, JUDEX_PREVIEW_ORIGIN: `http://localhost:${port}`,
    JUDEX_S3_ENDPOINT: `http://127.0.0.1:${s3Port}`, JUDEX_S3_ACCESS_KEY: "judex", JUDEX_S3_SECRET_KEY: password, JUDEX_S3_BUCKET: "judex", JUDEX_S3_PATH_STYLE: "true",
    JUDEX_REGISTER_PER_IP: "1000", JUDEX_MODEL_CATALOG_FILE: "", JUDEX_MODEL_PROTOCOL: "", JUDEX_OPENSANDBOX_ENDPOINT: "",
  };
  execFileSync(executable, ["objects", "init"], { env, windowsHide: true });
  const log = fs.openSync(path.join(artifact, "server.log"), "w");
  server = spawn(executable, [], { cwd: root, env, windowsHide: true, stdio: ["ignore", log, log] });
  await waitFor(base + "/readyz");
  run(process.execPath, [require.resolve("@playwright/test/cli"), "test", "--config", "tests/e2e/business.config.ts", ...process.argv.slice(2)], { env: { ...env, JUDEX_E2E_BASE_URL: base, JUDEX_E2E_CLI: cli, JUDEX_E2E_ARTIFACT: artifact, JUDEX_E2E_PG_CONTAINER:pg, JUDEX_E2E_RUN_ID:runId }, stdio: "inherit" });
  // Stop API and workers before capturing a consistent business baseline.
  await new Promise((resolve) => { server.once("exit", resolve); server.kill(); });
  const baseline = JSON.parse(run(executable, ["verify-recovery"], { env }));
  const dump = execFileSync("docker", ["exec", pg, "pg_dump", "-U", "postgres", "-Fc", "judex"], { windowsHide: true, maxBuffer: 128 * 1024 * 1024 });
  fs.writeFileSync(path.join(artifact, "judex.pgdump"), dump);
  docker("exec", pg, "createdb", "-U", "postgres", "restored");
  execFileSync("docker", ["exec", "-i", pg, "pg_restore", "-U", "postgres", "-d", "restored", "--exit-on-error"], { input: dump, windowsHide: true });
  // Exercise the installed backup command against real S3 and an empty restore bucket.
  const archive = path.join(artifact, "objects.tar");
  execFileSync(executable, ["objects", "export"], { env, cwd: root, windowsHide: true, stdio: ["ignore", fs.openSync(archive, "w"), "inherit"] });
  execFileSync(executable, ["objects", "init"], { env: { ...env, JUDEX_S3_BUCKET: "restored" }, windowsHide: true });
  execFileSync(executable, ["objects", "import"], { env: { ...env, JUDEX_S3_BUCKET: "restored" }, windowsHide: true, stdio: [fs.openSync(archive, "r"), "inherit", "inherit"] });
  const restored = JSON.parse(run(executable, ["verify-recovery"], { env: { ...env, JUDEX_S3_BUCKET: "restored", JUDEX_DATABASE_URL: env.JUDEX_DATABASE_URL.replace("/judex?", "/restored?") } }));
  if (JSON.stringify(restored) !== JSON.stringify(baseline)) throw new Error("Recovered database/material evidence differs from source");
  fs.writeFileSync(path.join(artifact, "recovery-evidence.json"), JSON.stringify(restored, null, 2));
  fs.writeFileSync(path.join(artifact, "manifest.json"), JSON.stringify({ runId, exitCode: 0, s3ArchiveRestoredAndVerified: true, postgresRestoredAndAllTablesMatched: true, verifiedMaterialEntries: restored.verifiedMaterialEntries }, null, 2));
  console.log(`Business acceptance passed; evidence: ${artifact}`);
} catch (error) {
  process.exitCode = 1;
  console.error(error.message.replaceAll(password, "[test credential]"));
  console.error(`Evidence: ${artifact}`);
} finally {
  if (server && server.exitCode == null) server.kill();
  for (const name of [pg, s3]) {
    try { const info = JSON.parse(docker("inspect", name))[0]; if (info.Config.Labels?.["judex.test-run"] === runId) docker("rm", "-f", name); } catch {}
  }
}
