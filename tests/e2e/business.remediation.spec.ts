import { expect, test, type BrowserContext, type Page } from "@playwright/test";
import { spawn, execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { createHash, randomUUID } from "node:crypto";

async function register(context: BrowserContext, name: string) {
  const response = await context.request.post("/api/v1/auth/register", { data: { displayName: name, email: `${randomUUID()}@judex.test`, password: "acceptance-password-123" } });
  expect(response.status()).toBe(201); return (await response.json()).data.user;
}
async function command(context: BrowserContext, route: string, body: unknown) {
  const session = (await (await context.request.get("/api/v1/auth/session")).json()).data;
  const response = await context.request.post("/api/v1" + route, { data: body, headers: { "X-CSRF-Token": session.csrfToken, "Idempotency-Key": randomUUID() } });
  expect(response.ok(), await response.text()).toBeTruthy(); return (await response.json()).data;
}
async function read(context: BrowserContext, route: string) {
  const response = await context.request.get("/api/v1" + route); expect(response.ok()).toBeTruthy(); return (await response.json()).data;
}
async function openProposal(page: Page, project: string) {
  await page.goto(`/?project=${project}`);
  await page.getByRole("button", { name: "work_arrangement", exact: true }).click();
  await expect(page.getByTestId("fixed-review")).toBeVisible();
}

test("R11/R12 双用户邀请、职责会签、交付与固定成果验收", async ({ browser }) => {
  const a = await browser.newContext(), b = await browser.newContext();
  try {
    const owner = await register(a, "验收人"), worker = await register(b, "交付人");
    const p = await command(a, "/projects", { title: "真实协作闭环" }); const prefix = `/projects/${p.id}`;
    const reviewerRole = await command(a, prefix + "/positions", { name: "验收职责", prompt: "验收成果" });
    const developerRole = await command(a, prefix + "/positions", { name: "开发职责", prompt: "开发交付" });
    const reviewer = await command(a, prefix + "/identities", { positionId: reviewerRole.id, userId: owner.id });
    const invite = await command(a, prefix + "/invitations", { targetEmail: worker.email, positionIds: [developerRole.id] });
    const bp = await b.newPage(); await bp.goto(invite.inviteUrl);
    await expect(bp.getByText("真实协作闭环", { exact: true })).toBeVisible();
    await bp.getByRole("button", { name: "接受邀请", exact: true }).click();
    await expect(bp.getByTestId("ws-nav-home")).toBeVisible();
    const identities = (await read(a, prefix + "/identities")).items;
    const developer = identities.find((i: any) => i.currentBinding?.userId === worker.id);
    const proposal = await command(a, prefix + "/proposals", { kind: "work_arrangement", changes: [
      { operation: "create_plan", targetType: "plan", clientRef: "p", fields: { title: "核对计划", goal: "保留证据", acceptanceCriteria: "完整交付", ownerIdentityId: reviewer.id } },
      { operation: "create_task", targetType: "task", clientRef: "t", fields: { title: "修复幂等", planId: "p", expectedOutput: "补齐重试证据", acceptanceCriteria: "重复请求不重复创建", reviewerIdentityId: reviewer.id, participantIdentityIds: [developer.id] } },
    ] });
    const draft = await read(a, `${prefix}/proposals/${proposal.id}/review`);
    await command(a, `${prefix}/proposals/${proposal.id}/submit`, { expectedVersion: 1, draftHash: draft.reviewHash });
    const ap = await a.newPage(); await openProposal(ap, p.id);
    await expect(ap.getByText("重复请求不重复创建", { exact: true })).toBeVisible();
    await ap.getByRole("button", { name: "同意", exact: true }).click();
    await expect.poll(async () => (await read(a, `${prefix}/proposals/${proposal.id}/review`)).status).toBe("pending");
    await openProposal(bp, p.id); await bp.getByRole("button", { name: "同意", exact: true }).click();
    await expect.poll(async () => (await read(a, `${prefix}/proposals/${proposal.id}/review`)).status).toBe("approved");
    const task = (await read(a, prefix + "/tasks")).items[0];
    await command(b, `${prefix}/tasks/${task.id}/start`, { expectedVersion: task.version, identityId: developer.id });
    const working = await read(b, `${prefix}/tasks/${task.id}`);
    await command(b, prefix + "/submissions", { clientSubmissionId: randomUUID(), purpose: "delivery", taskId: task.id, identityId: developer.id, expectedTaskVersion: working.version, text: "已通过八个并发请求，数据库只有一条成果记录。" });
    await ap.reload(); await ap.getByRole("button", { name: "修复幂等 · delivered", exact: true }).click();
    await ap.getByRole("button", { name: "验收", exact: true }).click();
    await expect(ap.getByText("已通过八个并发请求，数据库只有一条成果记录。", { exact: true })).toBeVisible();
    expect((await read(a, `${prefix}/tasks/${task.id}`)).status).toBe("delivered");
    await ap.getByRole("button", { name: "确认决定", exact: true }).click();
    await expect.poll(async () => (await read(a, `${prefix}/tasks/${task.id}`)).status).toBe("accepted");
    await ap.getByRole("button", {name:"核对计划",exact:true}).click();
    await ap.getByRole("button", {name:"整体验收计划",exact:true}).click();
    await ap.getByRole("button", {name:"确认决定",exact:true}).click();
    await expect.poll(async () => (await read(a,prefix+"/plans")).items[0].status).toBe("accepted");

  } finally { await a.close(); await b.close(); }
});

test("R09 真实 S3 多分片上传下载保持 SHA256", async ({ browser }) => {
  const ctx = await browser.newContext();
  try {
    await register(ctx, "材料验收"); const p = await command(ctx, "/projects", { title: "材料完整性" });
    const page = await ctx.newPage(); await page.goto(`/?project=${p.id}`); await page.getByTestId("ws-panel-materials").click();
    const content = Buffer.alloc(9 * 1024 * 1024 + 19, 65); content.write("最后一片也必须保留", content.length - 40);
    await page.locator('input[type="file"]').setInputFiles({ name: "完整报告.txt", mimeType: "text/plain", buffer: content });
    await page.getByRole("button", { name: "完整报告.txt", exact: true }).click({ timeout: 30000 });
    const download = page.getByRole("link", { name: "下载", exact: true }); await expect(download).toBeVisible();
    const response = await ctx.request.get((await download.getAttribute("href"))!);
    expect(response.status()).toBe(200); const bytes = await response.body();
    expect(bytes.length).toBe(content.length); expect(createHash("sha256").update(bytes).digest("hex")).toBe(createHash("sha256").update(content).digest("hex"));
  } finally { await ctx.close(); }
});

test("R20 HTML 包在独立 origin 加载相对 CSS 和脚本", async ({ browser }) => {
  const ctx = await browser.newContext();
  try {
    await register(ctx, "预览验收"); const p = await command(ctx, "/projects", { title: "HTML 预览" });
    const page = await ctx.newPage(); await page.goto(`/?project=${p.id}`); await page.getByTestId("ws-panel-materials").click();
    await page.getByText("HTML / ZIP", { exact: true }).click();
    await page.locator('input[type="file"]').setInputFiles("tests/fixtures/preview.zip");
    await page.getByRole("button", { name: "preview.zip", exact: true }).click({ timeout: 20000 });
    await page.getByRole("button", { name: "预览", exact: true }).click();
    const link = page.getByRole("link", { name: "预览", exact: true }); await expect(link).toBeVisible();
    const preview = await ctx.newPage(); await preview.goto((await link.getAttribute("href"))!);
    await expect(preview.getByRole("heading", { name: "隔离 HTML 预览" })).toHaveCSS("color", "rgb(12, 34, 56)");
    await expect(preview.getByRole("heading")).toHaveAttribute("data-executed", "yes");
    const replay = await ctx.request.get((await link.getAttribute("href"))!, { maxRedirects: 0 });
    expect(replay.status()).toBe(404);
    expect(new URL(preview.url()).origin).not.toBe(new URL(process.env.JUDEX_E2E_BASE_URL!).origin);
    expect((await ctx.cookies(preview.url())).some((c) => c.name.includes("session"))).toBe(false);
  } finally { await ctx.close(); }
});

test("R11 实际 CLI 登录必须由浏览器选择项目授权", async ({ browser }) => {
  const ctx = await browser.newContext(); let cli: ReturnType<typeof spawn> | undefined;
  try {
    const user = await register(ctx, "CLI 授权人"); const p = await command(ctx, "/projects", { title: "设备授权项目" });
    const config = path.join(process.env.JUDEX_E2E_ARTIFACT!, "cli-config"); fs.mkdirSync(config, { recursive: true });
    cli = spawn(process.env.JUDEX_E2E_CLI!, ["--server", process.env.JUDEX_E2E_BASE_URL!, "--json", "auth", "login", "--scopes", "projects:read,context:read,intents:create,materials:read,materials:write,events:read"], { env: { ...process.env, JUDEX_CONFIG_DIR: config, JUDEX_TOKEN: "" }, windowsHide: true });
    let stderr = "", stdout = ""; cli.stderr!.on("data", (v) => { stderr += v; }); cli.stdout!.on("data", (v) => { stdout += v; });
    await expect.poll(() => /输入代码：([^\s]+)/.exec(stderr)?.[1], { timeout: 10000 }).toBeTruthy();
    const code = /输入代码：([^\s]+)/.exec(stderr)![1]; const page = await ctx.newPage(); await page.goto("/device");
    await page.getByLabel("设备码", { exact: true }).fill(code); await page.getByRole("button", { name: "查看授权请求", exact: true }).click();
    for (const scope of ["projects:read", "context:read", "intents:create", "materials:read", "materials:write", "events:read"]) await page.getByText(scope, { exact: true }).click();
    await page.getByText("设备授权项目", { exact: true }).click();
    await page.getByRole("button", { name: "确认授权", exact: true }).click();
    await expect.poll(() => cli!.exitCode, { timeout: 15000 }).toBe(0);
    expect(JSON.parse(stdout).ok).toBe(true);
    const grants = (await read(ctx, "/me/client-grants")).items; expect(grants.some((g: any) => g.projectScope?.includes(p.id))).toBe(true);
    const invoke = (...args: string[]) => JSON.parse(execFileSync(process.env.JUDEX_E2E_CLI!, ["--server", process.env.JUDEX_E2E_BASE_URL!, "--json", ...args], { env: { ...process.env, JUDEX_CONFIG_DIR: config, JUDEX_TOKEN: "" }, windowsHide: true, encoding: "utf8" })).data;
    invoke("project", "use", p.id);
    const selected=path.join(config,"selected.txt");const content=Buffer.from("CLI 不可变材料。".repeat(1000));fs.writeFileSync(selected,content);
    const uploaded=invoke("material","upload",selected);const downloaded=path.join(config,"downloaded.txt");invoke("material","download","--version",uploaded.id,"--output",downloaded);expect(fs.readFileSync(downloaded)).toEqual(content);
    const bundle=path.join(config,"bundle");fs.mkdirSync(bundle);fs.writeFileSync(path.join(bundle,"index.html"),"<!doctype html><h1>CLI 包</h1>");fs.writeFileSync(path.join(bundle,"style.css"),"h1 { color: blue; }");
    const manifest=invoke("bundle","upload",bundle,"--dry-run");expect(manifest.files).toHaveLength(2);
    const version=invoke("bundle","upload",bundle);const archive=path.join(config,"downloaded.zip");const fetched=invoke("material","download","--version",version.id,"--output",archive);expect(fetched.verified).toBe(true);expect(createHash("sha256").update(fs.readFileSync(archive)).digest("hex")).toBe(version.sha256);
    const events=invoke("events","watch","--limit","1","--timeout","5s");expect(events.items).toHaveLength(1);expect(events.nextCursor).toBeGreaterThan(0);

    const prefix = `/projects/${p.id}`;
    const position = await command(ctx, prefix + "/positions", { name: "执行与验收", prompt: "核对成果" });
    const identity = await command(ctx, prefix + "/identities", { positionId: position.id, userId: user.id });
    const proposal = await command(ctx, prefix + "/proposals", { kind: "work_arrangement", changes: [{ operation: "create_task", targetType: "task", fields: { title: "CLI 一次确认", expectedOutput: "已验证成果", acceptanceCriteria: "只写一条验收记录", reviewerIdentityId: identity.id, participantIdentityIds: [identity.id] } }] });
    const draft = await read(ctx, `${prefix}/proposals/${proposal.id}/review`);
    const review = await command(ctx, `${prefix}/proposals/${proposal.id}/submit`, { expectedVersion: 1, draftHash: draft.reviewHash });
    await command(ctx, `${prefix}/proposals/${proposal.id}/decisions`, { reviewId: review.reviewId, reviewHash: review.reviewHash, expectedVersion: 2, decision: "approve", slotIds: review.slots.filter((s: any) => s.canDecide).map((s: any) => s.id), actingBindingVersions: review.slots.filter((s: any) => s.canDecide && s.authorityType === "identity").map((s: any) => ({ identityId: s.authorityId, bindingVersion: s.bindingVersion })) });
    const task = (await read(ctx, prefix + "/tasks")).items[0];
    await command(ctx, `${prefix}/tasks/${task.id}/reports`, { kind: "delivery", text: "固定成果：请求完成后只验收一次。", identityId: identity.id, expectedTaskVersion: task.version });
    const intent = invoke("--no-wait", "task", "accept", task.id);
    await page.goto(intent.confirmUrl);
    await expect(page.getByText("固定成果：请求完成后只验收一次。", { exact: true })).toBeVisible();
    expect((await read(ctx, `${prefix}/tasks/${task.id}`)).status).toBe("delivered");
    await page.getByTestId("confirm-approve").click();
    await expect.poll(() => invoke("decision", "result", intent.id).state).toBe("committed");
    const first = invoke("decision", "result", intent.id);
    await command(ctx, `${prefix}/confirmation-intents/${intent.id}/confirm`, { decision: "approve", intentHash: intent.nonce });
    expect(invoke("decision", "result", intent.id).resultRef).toBe(first.resultRef);
    const audit = (await read(ctx, prefix + "/audit")).items.filter((row: any) => row.operation === "task.accept" && row.objectId === task.id);
    expect(audit).toHaveLength(1); expect(audit[0].source).toBe("cli");

  } finally { if (cli && cli.exitCode === null) cli.kill(); await ctx.close(); }
});


test("研发记录：Bug 详情、发布事实与独立修复草稿",async({browser})=>{
 const ctx=await browser.newContext();try {
  const user=await register(ctx,"研发验收");const p=await command(ctx,"/projects",{title:"研发事实"});const prefix=`/projects/${p.id}`;
  const role=await command(ctx,prefix+"/positions",{name:"研发职责",prompt:"核对事实"});const identity=await command(ctx,prefix+"/identities",{positionId:role.id,userId:user.id});
  const bug=await command(ctx,prefix+"/tasks",{title:"复现异常",kind:"bug",participantIdentityIds:[identity.id],reviewerIdentityId:identity.id,bugDetails:{environment:"测试",steps:"打开列表",expected:"正常加载",actual:"空白",severity:"high"}});
  expect((await read(ctx,prefix+`/tasks/${bug.id}`)).bugDetails.actual).toBe("空白");
  const page=await ctx.newPage();await page.goto(`/?project=${p.id}`);await page.getByRole("button",{name:"研发记录",exact:true}).click();
  const form=page.locator('form').filter({has:page.getByRole('button',{name:'记录发布事实',exact:true})});
  await form.getByLabel('版本',{exact:true}).fill('v1.2.3');await form.getByLabel('环境',{exact:true}).fill('staging');await form.getByRole('button',{name:'记录发布事实',exact:true}).click();
  await expect.poll(async()=>(await read(ctx,prefix+'/release-reports')).items.length).toBe(1);
  await page.getByRole('button',{name:/Bug$/}).click();await page.getByRole('option',{name:'复现异常',exact:true}).click();await page.getByText('v1.2.3 · staging',{exact:true}).click();await page.getByRole('button',{name:'创建修复传播草稿',exact:true}).click();
  await expect(page.getByRole('status').filter({hasText:'已创建'})).toBeVisible();
  const tasks=(await read(ctx,prefix+'/tasks')).items;expect(tasks).toHaveLength(2);expect(tasks.every((t:any)=>t.status==='draft')).toBe(true);
 }finally{await ctx.close()}
});


test("流程编辑器保存强制岗位、委托授权并明确发布",async({browser})=>{
 const ctx=await browser.newContext();try{
  const user=await register(ctx,"流程负责人");const p=await command(ctx,"/projects",{title:"流程约束验收"});const prefix=`/projects/${p.id}`;
  await command(ctx,prefix+"/positions",{name:"必须会签岗位",prompt:"检查前置证据"});
  const page=await ctx.newPage();await page.goto(`/?project=${p.id}&settings=project`);
  await page.getByRole("button",{name:"新建流程",exact:true}).click();const editor=page.getByTestId("workflow-editor");
  await editor.getByLabel("名称",{exact:true}).fill("发布前证据检查");await editor.getByRole("button",{name:"添加节点",exact:true}).click();
  await editor.getByLabel("名称",{exact:true}).nth(1).fill("验收节点");await editor.getByLabel("节点职责",{exact:true}).fill("核验事实，不代替人工验收");
  await editor.getByText("必须会签岗位",{exact:true}).click();await editor.getByText("流程负责人",{exact:true}).click();
  await editor.getByRole("button",{name:"保存流程草稿",exact:true}).click();await expect(editor).not.toBeVisible();
  const workflows=(await read(ctx,prefix+"/workflows")).items;expect(workflows).toHaveLength(1);expect(workflows[0].publishedVersionId).toBeNull();
  await page.getByRole("button",{name:"发布前证据检查",exact:true}).click();await page.getByRole("button",{name:/核对流程版本.*draft/}).click();
  const review=page.getByTestId("workflow-review");await expect(review.getByText(/必须会签的岗位.*必须会签岗位/)).toBeVisible();
  await review.getByRole("button",{name:"确认发布",exact:true}).click();
  await expect.poll(async()=>(await read(ctx,prefix+"/workflows")).items[0].publishedVersionId).toBeTruthy();
  const version=(await read(ctx,prefix+`/workflows/${workflows[0].id}/versions`)).items[0];expect(version.body.nodes[0].delegationUserIds).toEqual([user.id]);
 }finally{await ctx.close()}
});


test("性能样本：千任务首屏分页与万消息范围查询",async({browser})=>{
 const ctx=await browser.newContext();try{
  const user=await register(ctx,"分页样本");const p=await command(ctx,"/projects",{title:"分页性能样本"});const prefix=`/projects/${p.id}`;
  const role=await command(ctx,prefix+"/positions",{name:"分页职责",prompt:"样本"});const identity=await command(ctx,prefix+"/identities",{positionId:role.id,userId:user.id});const topic=(await read(ctx,prefix+"/topics")).items[0];
  for(const id of [p.id,identity.id,topic.id,user.id]) expect(id).toMatch(/^[0-9a-f-]{36}$/);
  const pg=process.env.JUDEX_E2E_PG_CONTAINER!;const info=JSON.parse(execFileSync('docker',['inspect',pg],{encoding:'utf8',windowsHide:true}))[0];expect(info.Config.Labels['judex.test-run']).toBe(process.env.JUDEX_E2E_RUN_ID);
  execFileSync('docker',['exec',pg,'psql','-U','postgres','-d','judex','-v','ON_ERROR_STOP=1','-c',`
   INSERT INTO tasks(project_id,id,title,kind,status,reviewer_identity_id,created_by,created_at,updated_at) SELECT '${p.id}',gen_random_uuid(),'性能任务 '||n,'task','ready','${identity.id}','${user.id}',now()-(1000-n)*interval '1 millisecond',now() FROM generate_series(1,1000)n;
   INSERT INTO task_participants(project_id,task_id,identity_id) SELECT project_id,id,'${identity.id}' FROM tasks WHERE project_id='${p.id}';
   INSERT INTO messages(project_id,id,topic_id,seq,kind,author_user_id,content,state,created_at) SELECT '${p.id}',gen_random_uuid(),'${topic.id}',n,'human','${user.id}','第 '||n||' 条原始中文讨论','committed',now() FROM generate_series(1,10000)n;
   UPDATE topics SET last_message_seq=10000 WHERE id='${topic.id}';`],{encoding:'utf8',windowsHide:true});
  const page=await ctx.newPage();const first=Date.now();await page.goto(`/?project=${p.id}`);await expect(page.getByRole('button',{name:'性能任务 1000 · ready',exact:true})).toBeVisible();const firstPageMs=Date.now()-first;
  await expect(page.getByRole('button',{name:/^性能任务 .* · ready$/})).toHaveCount(50);await page.getByRole('button',{name:'加载更多 · 任务',exact:true}).click();await expect(page.getByRole('button',{name:/^性能任务 .* · ready$/})).toHaveCount(100);
  const messagesStart=Date.now();const messages=await read(ctx,`${prefix}/topics/${topic.id}/messages?limit=50&afterSeq=9950`);expect(messages.items).toHaveLength(50);expect(messages.items[0].seq).toBe(9951);expect(messages.items[49].seq).toBe(10000);
  const memory=await page.evaluate(()=> (performance as Performance & {memory?:{usedJSHeapSize:number}}).memory?.usedJSHeapSize??null);
  fs.writeFileSync(path.join(process.env.JUDEX_E2E_ARTIFACT!,"performance.json"),JSON.stringify({tasks:1000,messages:10000,firstPageRows:50,rowsAfterLoadMore:100,firstPageMs,messagePageMs:Date.now()-messagesStart,usedJSHeapBytes:memory},null,2));
 }finally{await ctx.close()}
});


test("CLI 首次建项目只准备全局意图，浏览器一次确认后才创建",async({browser})=>{
 const ctx=await browser.newContext();let processHandle:ReturnType<typeof spawn>|undefined;
 try{
  await register(ctx,"首次建项目用户");const config=path.join(process.env.JUDEX_E2E_ARTIFACT!,"global-cli");fs.mkdirSync(config,{recursive:true});
  const env={...process.env,JUDEX_CONFIG_DIR:config,JUDEX_TOKEN:""};processHandle=spawn(process.env.JUDEX_E2E_CLI!,["--server",process.env.JUDEX_E2E_BASE_URL!,"--json","auth","login","--scopes","projects:read,projects:create,intents:create"],{env,windowsHide:true});
  let stderr="";processHandle.stderr!.on("data",(data)=>stderr+=data);await expect.poll(()=>/输入代码：([^\s]+)/.exec(stderr)?.[1]).toBeTruthy();
  const page=await ctx.newPage();await page.goto("/device");await page.getByLabel("设备码",{exact:true}).fill(/输入代码：([^\s]+)/.exec(stderr)![1]);await page.getByRole("button",{name:"查看授权请求",exact:true}).click();
  for(const scope of ["projects:read","projects:create","intents:create"])await page.getByText(scope,{exact:true}).click();
  await page.getByRole("button",{name:"确认授权",exact:true}).click();await expect.poll(()=>processHandle!.exitCode,{timeout:15000}).toBe(0);
  const invoke=(...args:string[])=>JSON.parse(execFileSync(process.env.JUDEX_E2E_CLI!,["--server",process.env.JUDEX_E2E_BASE_URL!,"--json",...args],{env,windowsHide:true,encoding:"utf8"})).data;
  const file=path.join(config,"project.json.input");fs.writeFileSync(file,JSON.stringify({title:"经本人确认建立",maxDiscussionRounds:4}));
  const intent=invoke("--no-wait","project","create","--file",file);expect((await read(ctx,"/projects")).items).toHaveLength(0);
  await page.goto(intent.confirmUrl);await expect(page.getByText("经本人确认建立",{exact:true})).toBeVisible();await page.getByTestId("confirm-approve").click();
  await expect.poll(()=>invoke("decision","result",intent.id,"--global").state).toBe("committed");const projects=invoke("project","list");expect(projects.items).toHaveLength(1);expect(projects.items[0].maxDiscussionRounds).toBe(4);
  await command(ctx,`/confirmation-intents/${intent.id}/confirm`,{decision:"approve",intentHash:intent.nonce});expect((await read(ctx,"/projects")).items).toHaveLength(1);
 }finally{if(processHandle?.exitCode===null)processHandle.kill();await ctx.close()}
});
