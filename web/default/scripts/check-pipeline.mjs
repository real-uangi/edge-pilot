import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { build } from "vite";

const root = fileURLToPath(new URL("..", import.meta.url));
const temporary = await mkdtemp(join(root, "node_modules/.pipeline-check-"));
const originalNow = Date.now;
const now = Date.parse("2026-11-15T12:00:00Z");
const at = (offset) => new Date(now + offset).toISOString();

try {
  const bundle = await build({
    root,
    logLevel: "silent",
    build: {
      ssr: join(root, "src/features/releases/components/TaskTimeline.tsx"),
      ssrEmitAssets: true,
      write: false,
    },
  });
  const entry = join(temporary, "pipeline.mjs");
  await writeFile(entry, bundle.output.find((item) => item.type === "chunk" && item.isEntry).code);
  const { TaskTimeline } = await import(pathToFileURL(entry).href);
  const stages = [
    { id: "dispatch", label: "派发", kind: "automatic", estimatedDurationMs: 1000, sampleCount: 3 },
    { id: "image", label: "镜像准备", kind: "automatic", estimatedDurationMs: 20000, sampleCount: 3 },
    { id: "container", label: "启动容器", kind: "automatic", estimatedDurationMs: 5000, sampleCount: 3 },
    { id: "health", label: "健康检查", kind: "automatic", estimatedDurationMs: null, sampleCount: 0 },
    { id: "confirmation", label: "等待确认", kind: "manual", estimatedDurationMs: null, sampleCount: 0 },
    { id: "traffic", label: "切换流量", kind: "event", estimatedDurationMs: null, sampleCount: 0 },
  ];
  function pipeline(stageId, patch, taskStatus = 3) {
    return {
      stages,
      runs: [{
        taskId: "task-current",
        sequence: 2,
        status: taskStatus,
        startedAt: at(-60000),
        completedAt: null,
        replayCount: 1,
        cells: stages.map((stage) => ({
          stageId: stage.id,
          status: "pending",
          durationMs: null,
          runningSince: null,
          events: [],
          ...(stage.id === stageId ? patch : {}),
        })),
      }],
    };
  }
  function render(data, time = now) {
    Date.now = () => time;
    return renderToStaticMarkup(createElement(TaskTimeline, { pipeline: data, tasks: [] }));
  }
  function cell(markup, status) {
    const result = markup.match(new RegExp(`<button[^>]*data-status="${status}"[^>]*>[\\s\\S]*?</button>`));
    assert.ok(result, `missing ${status} cell`);
    return result[0];
  }

  const running = render(pipeline("image", {
    status: "running", durationMs: 5000, runningSince: at(-10000),
  }));
  assert.match(running, /<table[^>]*aria-labelledby="pipeline-heading"/);
  assert.match(running, /重新派发 1 次/);
  assert.match(running, /查看第 2 次执行的任务日志/);
  assert.match(cell(running, "running"), /15s/);
  assert.match(cell(running, "running"), /aria-valuenow="75"/);

  const over = render(pipeline("image", {
    status: "running", durationMs: 5000, runningSince: at(-25000),
  }));
  assert.match(cell(over, "running"), /超出预计/);
  assert.match(cell(over, "running"), /aria-valuenow="95"/);
  assert.match(cell(over, "running"), /scaleX\(0\.95\)/);

  const complete = pipeline("image", { status: "completed", durationMs: 13500 }, 4);
  assert.equal(render(complete), render(complete, now + 3600000), "completed history must stop counting");
  assert.match(render(complete), /部署成功/);
  assert.match(cell(render(complete), "completed"), /aria-valuenow="100"/);

  const failed = pipeline("image", { status: "failed", durationMs: 3000 }, 5);
  assert.equal(render(failed), render(failed, now + 3600000), "failed history must stop counting");
  assert.match(cell(render(failed), "failed"), /失败/);
  assert.match(cell(render(failed), "failed"), /aria-valuenow="15"/);
  assert.doesNotMatch(cell(render(failed), "failed"), /aria-valuenow="100"/);
  const failedUnknown = cell(render(pipeline("health", {
    status: "timed_out", durationMs: 90000,
  }, 6)), "timed_out");
  assert.match(failedUnknown, /data-error-without-estimate="true"/);
  assert.doesNotMatch(failedUnknown, /role="progressbar"/);
  assert.doesNotMatch(failedUnknown, /indeterminate/);

  const noEstimate = cell(render(pipeline("health", {
    status: "running", durationMs: 0, runningSince: at(-5000),
  })), "running");
  assert.match(noEstimate, /indeterminate/);
  assert.doesNotMatch(noEstimate, /aria-valuenow=/);

  const waiting = cell(render(pipeline("confirmation", {
    status: "waiting", durationMs: 0, runningSince: at(-120000),
  }, 4)), "waiting");
  assert.match(waiting, /2m 0s/);
  assert.match(waiting, /等待确认/);
  assert.doesNotMatch(waiting, /role="progressbar"/);
  assert.doesNotMatch(waiting, /indeterminate/);

  const pending = render(pipeline("dispatch", { status: "waiting", durationMs: null }, 1));
  assert.match(cell(pending, "waiting"), /等待派发/);
  assert.doesNotMatch(pending, /role="progressbar"/);
  assert.match(render({ stages, runs: [] }), /暂无执行记录/);

  const twoRuns = pipeline("health", { status: "running", durationMs: 0, runningSince: at(-1000) });
  twoRuns.runs.push({ ...complete.runs[0], taskId: "task-previous", sequence: 1 });
  const rows = render(twoRuns);
  assert.ok(rows.indexOf("第 2 次执行") < rows.indexOf("第 1 次执行"));
  if (process.argv[2]) {
    const preview = pipeline("health", { status: "running", durationMs: 0, runningSince: at(-18000) });
    preview.stages = stages.map((stage) => stage.id === "health" ? { ...stage, estimatedDurationMs: 15000, sampleCount: 3 } : stage);
    for (const [index, durationMs] of [1200, 14000, 3000].entries()) {
      Object.assign(preview.runs[0].cells[index], { status: "completed", durationMs });
    }
    const previous = pipeline("health", { status: "failed", durationMs: 90000 }, 5).runs[0];
    Object.assign(previous, { taskId: "task-previous", sequence: 1, replayCount: 0 });
    for (const [index, durationMs] of [1000, 16000, 4000].entries()) {
      Object.assign(previous.cells[index], { status: "completed", durationMs });
    }
    preview.runs.push(previous);
    const css = [await readFile(join(root, "src/styles/global.css"), "utf8"),
      ...bundle.output.filter((item) => item.type === "asset" && item.fileName.endsWith(".css")).map((item) => item.source),
    ].join("\n");
    await writeFile(process.argv[2], `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>流水线阶段表静态检查</title><style>${css}body{padding:24px}main{max-width:1440px;min-width:0;margin:auto} @media(max-width:767px){body{padding:16px}}</style><main>${render(preview)}</main><script>const wrapper=document.querySelector('[role="region"]');const cell=document.querySelector('tbody td');const metrics={viewport:innerWidth,pageWidth:document.documentElement.scrollWidth,tableWidth:wrapper.scrollWidth,wrapperWidth:wrapper.clientWidth,cellHeight:cell.clientHeight,buttonHeight:cell.firstElementChild.offsetHeight};document.body.dataset.layout=JSON.stringify(metrics);</script></html>`);
    console.log(`Wrote static layout preview to ${process.argv[2]}`);
  }
  console.log("Checked pipeline table, elapsed-time accumulation, progress cap, stopped history, manual wait, no estimates, replay and retry rows.");
} finally {
  Date.now = originalNow;
  await rm(temporary, { recursive: true, force: true });
}
