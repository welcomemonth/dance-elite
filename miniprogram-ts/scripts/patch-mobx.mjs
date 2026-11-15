import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

// 微信小程序没有 Node 的 process 对象，mobx 的 dist/index.js 在运行时读取
// process.env.NODE_ENV 会报 "process is not defined"。这里按 Annil 官方文档
// 的做法，直接固定为 production 构建。
const root = dirname(dirname(fileURLToPath(import.meta.url)));
const entry = join(root, "node_modules/mobx/dist/index.js");

if (!existsSync(entry)) {
  process.exit(0);
}

const source = readFileSync(entry, "utf8");
if (!source.includes("process.env.NODE_ENV")) {
  process.exit(0);
}

writeFileSync(
  entry,
  `'use strict'\n\nmodule.exports = require('./mobx.cjs.production.min.js')\n`,
  "utf8",
);
console.log("[patch-mobx] mobx 已切换为 production 构建（小程序端无 process 对象）");
