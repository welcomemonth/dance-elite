# 优舞盟小程序模板（Annil + TypeScript）

基于 [Annil](https://missannil.github.io/annil/) 的微信原生小程序 TypeScript 模板：保留原生目录、WXML 与 WXSS，用 TypeScript 编写页面/组件逻辑，并提供类型安全的组件模型、MobX 全局状态和统一请求层。

- Annil `1.18.x`：`DefineComponent` / `RootComponent` / `CustomComponent` / `ChunkComponent` / `store` / `navigateTo`
- TypeScript `6.0.x` + `miniprogram-api-typings`（原生 API 类型）
- MobX `6.x`（Annil `store` 的响应式数据源）
- 微信原生小程序，无第三方 UI 库

## 快速开始

1. 安装依赖（`postinstall` 会把 mobx 入口固定为 production 构建，小程序端没有 `process` 对象）：

   ```bash
   pnpm install
   ```

2. 用微信开发者工具导入 **`miniprogram-ts/` 目录**（不是仓库里的 `miniprogram/`）。
   - `project.config.json` 已配置 `miniprogramRoot: "miniprogram/"` 与 TypeScript 编译器插件
   - AppID 当前为 `touristappid`，替换成自己的 AppID

3. 在开发者工具中执行 **工具 → 构建 npm**，生成 `miniprogram/miniprogram_npm/`。
   - 这一步是**必须**的：TS 编译只处理 `.ts` 源码，不会打包 `annil` / `mobx`；未构建时运行会报 `module 'annil.js' is not defined, require args is 'annil'`
   - 构建成功后应能看到 `miniprogram/miniprogram_npm/annil/` 与 `miniprogram/miniprogram_npm/mobx/`

4. 配置接口地址：`miniprogram/config/env.ts`
   - `develop` 默认 `http://localhost:8080`，本地联调需在开发者工具中勾选「不校验合法域名」
   - `trial` / `release` 改成线上域名

5. 类型检查 / 监听：

   ```bash
   pnpm typecheck   # tsc --noEmit
   pnpm watch       # tsc --noEmit --watch
   ```

> TypeScript 由开发者工具的 `useCompilerPlugins: ["typescript"]` 编译，不需要额外的打包步骤；但编译器插件不做类型检查，提交前请跑 `pnpm typecheck`。

## 目录结构

```
miniprogram-ts/
├── miniprogram/
│   ├── app.ts / app.json / app.wxss   # 入口、全局样式
│   ├── inject.ts                      # instanceConfig 全局注入 + IInjectInfo 类型增强
│   ├── config/env.ts                  # 运行环境与 API 地址
│   ├── types/                         # 响应包装、错误码、领域模型
│   ├── utils/                         # request / session / toast / api-error
│   ├── api/                           # 按模块拆分的接口函数
│   ├── store/                         # MobX 全局状态
│   ├── components/empty-state/        # 自定义组件示例（props + customEvents）
│   └── pages/home/                    # 页面示例（store / computed / 注入方法）
├── typings/index.d.ts                 # IAppOption 等全局声明
├── scripts/patch-mobx.mjs             # postinstall 处理 mobx 的小程序兼容问题
├── project.config.json
├── tsconfig.json
└── pnpm-workspace.yaml                # nodeLinker: hoisted，兼容「构建 npm」
```

## 开发约定

### 页面与组件

- **页面**：`RootComponent()({ isPage: true, ... })` + `DefineComponent({ path })`；`path` 必须与 `app.json` 的 `pages` 一致，并导出 `$Xxx` 页面文档类型。
- **组件**：`RootComponent()` + `DefineComponent({ name })`，导出组件文档类型并用 `typeEqual` 校验（见 `components/empty-state`）。
- **父组件接入**：`CustomComponent<Root, $Comp>()`；字段前缀由组件名推导，例如 `emptyState_text`、`emptyState_retry`。
- **逻辑片段**：不值得拆成组件的 WXML 片段用 `ChunkComponent` 组织，不需要注册 `usingComponents`。

### 请求层

`utils/request.ts` 提供 `http.get/post/put/delete`：

- 自动拼接 `API_BASE_URL`，默认携带 `Authorization: Bearer <token>`（`auth: false` 可跳过）
- 解包 `{ code, message, data }`，调用方直接拿到 `data`
- 失败统一抛 `ApiError`（`code` 为业务码）；`40101` 会触发 `setUnauthorizedHandler` 注册的全局处理
- 分页响应用 `PagedList<T>`
- 业务错误码常量集中在 `types/api.ts`（含报名域 `41001–41015`）

### 登录态

- `app.ts` 的 `onLaunch` 发起微信静默登录：`wx.login` → `POST /api/v1/auth/wx-login`
- 页面用 `userStore.ensureLogin()`；并发调用会复用同一个 Promise
- 登录态持久化在 storage（key：`dance_elite_session`），冷启动时直接恢复

### 全局注入

`inject.ts` 通过 `instanceConfig.setInjectInfo` 注入所有页面/组件可用的 `data`（`appName`、`appVersion`、`envVersion`）和方法（`showToast`、`showError`），并通过 `declare module "annil"` 补全类型提示。新增注入字段时同时更新 `injectData` / `injectMethods` 即可。

## 新增页面清单

1. 在 `miniprogram/pages/<name>/` 下创建 `.ts` / `.json` / `.wxml` / `.wxss`
2. `.ts` 用 `DefineComponent({ path: "/pages/<name>/<name>" })` 定义并在 `app.json` 注册
3. `.json` 中声明 `usingComponents`
4. 跳转使用 `navigateTo<$Page>({ url, data })`，参数类型会被检查
5. 完成后执行 `pnpm typecheck`

## 常见问题

- **`module 'annil.js' is not defined, require args is 'annil'`**：没有执行「构建 npm」，或 `node_modules` 不完整。先在 `miniprogram-ts/` 下 `pnpm install`，再执行 工具 → 构建 npm，确认生成 `miniprogram/miniprogram_npm/annil/`。
- **构建 npm 提示找不到 `node_modules`**：确认导入的是 `miniprogram-ts/` 目录，且 `pnpm install` 在项目根目录执行；`pnpm-workspace.yaml` 的 `nodeLinker: hoisted` 不能删。
- **运行时报 `process is not defined`**：mobx 仍指向 development 构建，重新执行 `pnpm install`（`postinstall` 会重写 mobx 入口）。
- **`annil` 找不到类型提示**：确认使用的是 VS Code + `vscode-annil` 插件，且 `node_modules/annil` 已安装。

## 说明

- 本模板是业务骨架，**不含 tabBar**；需要底部导航时在 `app.json` 中增加 `tabBar` 与对应页面。
- `node_modules/`、`miniprogram/miniprogram_npm/`、`project.private.config.json` 不入库。
- `pnpm-workspace.yaml` 中的 `nodeLinker: hoisted` 不能删：微信开发者工具「构建 npm」不识别 pnpm 的符号链接结构，需要与 npm 一致的扁平 `node_modules`。
- `tsconfig.json` 按 Annil 官方要求关闭了 `strictFunctionTypes`，否则事件回调的逆变写法会误报。
- 建议在 VS Code 安装 `vscode-annil` 插件，获得 WXML/JSON/TS 的组件字段检查与代码补全。
