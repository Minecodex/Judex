// P6-03 环境准备：单容器 PG + MinIO + judex-server（生产 web dist）。
// 幂等：复用已存在的容器；退出时清理。
import { execSync, spawn } from "node:child_process";

const PG = "judex-busy-pg";
const MINIO = "judex-busy-minio";
const SERVER_PORT = 18090;
const run = (cmd) => execSync(cmd, { stdio: ["ignore", "pipe", "ignore"] }).toString().trim();

function ensureContainer(name, args) {
  let exists = false;
  try { run(`docker inspect ${name}`); exists = true; } catch { exists = false; }
  if (exists) {
    try { run(`docker start ${name}`); } catch { /* already running */ }
    return;
  }
  run(`docker run -d --name ${name} ${args}`);
}

async function main() {
  // 幂等：已就绪的服务直接复用（端口占用时旧进程仍以磁盘静态文件服务新 dist）。
  try {
    const existing = await fetch(`http://127.0.0.1:${SERVER_PORT}/readyz`);
    if (existing.ok) {
      console.log(`business e2e server already ready on :${SERVER_PORT}`);
      return;
    }
  } catch { /* not running */ }
  // 生产 bundle 必须与当前源码一致（build:demo 会把 dist 覆盖为演示包）。
  execSync("npm run build", { cwd: new URL("../../web/", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"), stdio: "inherit" });
  ensureContainer(PG, "-e POSTGRES_PASSWORD=busy -e POSTGRES_DB=judex -p 127.0.0.1::5432 postgres:17-alpine");
  const minioImage = (() => {
    try {
      const lines = run(`docker images --format "{{.Repository}}:{{.Tag}} {{.ID}}"`).split(String.fromCharCode(10));
      return lines.find((l) => l.includes("minio") && !l.includes("mc")) ?? "";
    } catch { return ""; }
  })();
  const minioRef = minioImage ? minioImage.split(" ")[1] : "quay.io/minio/minio:latest";
  ensureContainer(MINIO, `--user root -e MINIO_ROOT_USER=judex -e MINIO_ROOT_PASSWORD=judex-busy -p 127.0.0.1::9000 ${minioRef} server /data`);
  // docker port is quoting-friendly on Windows cmd (inspect --format is not).
  const pgPort = run(`docker port ${PG} 5432`).split(":").pop().trim();
  const s3Port = run(`docker port ${MINIO} 9000`).split(":").pop().trim();
  try {
    run(`docker exec ${MINIO} sh -c "mc alias set local http://127.0.0.1:9000 judex judex-busy && mc mb local/judex --ignore-existing"`);
  } catch { /* bucket may exist */ }
  const server = spawn("go", ["run", "./cmd/judex-server"], {
    cwd: new URL("../../", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1"),
    env: {
      ...process.env,
      JUDEX_ENV: "development",
      JUDEX_HTTP_ADDR: `127.0.0.1:${SERVER_PORT}`,
      JUDEX_WEB_DIR: "web/dist",
      JUDEX_DATABASE_URL: `postgres://postgres:busy@127.0.0.1:${pgPort}/judex?sslmode=disable`,
      JUDEX_ALLOWED_ORIGINS: `http://127.0.0.1:${SERVER_PORT}`,
      JUDEX_S3_ENDPOINT: `http://127.0.0.1:${s3Port}`,
      JUDEX_S3_ACCESS_KEY: "judex",
      JUDEX_S3_SECRET_KEY: "judex-busy",
      JUDEX_S3_BUCKET: "judex",
      JUDEX_S3_PATH_STYLE: "true",
    },
    stdio: ["ignore", "pipe", "pipe"],
  });
  server.stdout.on("data", (d) => process.env.JUDEX_E2E_VERBOSE && process.stderr.write(d));
  server.stderr.on("data", (d) => process.stderr.write(d));
  // Wait for readiness.
  const deadline = Date.now() + 60000;
  while (Date.now() < deadline) {
    try {
      const resp = await fetch(`http://127.0.0.1:${SERVER_PORT}/readyz`);
      if (resp.ok) break;
    } catch { /* retry */ }
    await new Promise((r) => setTimeout(r, 500));
  }
  console.log(`business e2e server ready on :${SERVER_PORT}`);
}

main().catch((e) => { console.error(e); process.exit(1); });
