[English](./README.en.md) | **简体中文**

<div align="center">

<img src="assets/icons/pin.svg" width="42" alt="tailpin" />

# tailpin

**把编码 Agent 长会话的最新结论，钉在终端顶部**

<img src="https://readme-typing-svg.demolab.com?font=Fira%20Code&size=24&pause=1300&color=4493F8&center=true&vCenter=true&width=640&height=44&lines=%E6%8A%8A%E7%BC%96%E7%A0%81%20Agent%20%E7%9A%84%E6%9C%80%E6%96%B0%E7%BB%93%E8%AE%BA%E9%92%89%E5%9C%A8%E5%B1%8F%E5%B9%95%E4%B8%8A;%E6%AF%8F%E6%9D%A1%E7%BB%93%E8%AE%BA%E4%B8%80%E9%94%AE%E8%B7%B3%E5%9B%9E%E5%8E%9F%E5%A7%8B%20transcript%20%E8%AE%B0%E5%BD%95;tailpin%20diff%20%E5%AF%B9%E6%AF%94%E4%B8%A4%E6%AC%A1%E8%BF%90%E8%A1%8C%E7%9A%84%E7%AD%94%E6%A1%88" alt="tailpin" />

[![Go Version](https://img.shields.io/badge/go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![CI](https://img.shields.io/github/actions/workflow/status/SuperMarioYL/tailpin/ci.yml?branch=main&label=CI)](https://github.com/SuperMarioYL/tailpin/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/SuperMarioYL/tailpin?include_prereleases)](https://github.com/SuperMarioYL/tailpin/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/SuperMarioYL/tailpin.svg)](https://pkg.go.dev/github.com/SuperMarioYL/tailpin)
[![Go Report Card](https://goreportcard.com/badge/github.com/SuperMarioYL/tailpin)](https://goreportcard.com/report/github.com/SuperMarioYL/tailpin)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](./LICENSE)

免费开源（MIT）· 严格只读 · 全离线确定性 · 无 API key · 无配置

</div>

---

## <img src="assets/icons/rocket.svg" width="20" /> 快速开始

安装（需要 Go 1.24+）：

```sh
go install github.com/SuperMarioYL/tailpin@latest
```

或从 [Releases](https://github.com/SuperMarioYL/tailpin/releases) 下载 macOS / Linux 二进制（每个版本附带 sha256 校验和）。

用仓库自带的两条示例会话跑通全流程，不需要装 Claude Code：

```sh
git clone https://github.com/SuperMarioYL/tailpin.git
cd tailpin/examples

# 1) 纯文本面板：答案立即置顶，每条结论带 L20 行号溯源
tailpin watch projects/-Users-dev-work-api-server/run-b.jsonl --plain

# 2) TUI：j/k 选择结论 · Enter 跳到产生它的原始记录 · esc 返回 · q 退出
tailpin watch projects/-Users-dev-work-api-server/run-b.jsonl

# 3) 跨会话 diff：同一任务两次运行，结论逐条对比
CLAUDE_CONFIG_DIR=. tailpin diff run-a run-b
```

`--plain` 的真实输出（示例会话 run-b，会话进行中实时刷新）：

```text
tailpin · d8e9f0a1-2222-4bbb-8ccc-000000000002 · rev 1 · high
examples/projects/-Users-dev-work-api-server/run-b.jsonl

  L20     In summary, the flaky parser test is fixed for good:
  L20     Root cause: CountTokens in internal/parser/parser.go double-counted the trailing newline via the i == len(s) sentinel iteration.
  L20     Fix: corrected the loop boundary at parser.go:41 and added a regression case to parser_test.go.
  L20     Verified: go test ./... passes, 214/214.
  L20     Caveat: the config loader refactor is still pending.
```

真实会话零参数启动——tailpin 自动选中 `~/.claude/projects` 下最新活跃的会话；也可以 watch 任何 Claude-Code 风格的 JSONL 文件（Opencode 等写出同方言的会话文件同样适用）：

```sh
tailpin                              # 自动发现最新会话（TUI）
tailpin watch path/to/session.jsonl  # 指定任意 Claude-Code 风格 JSONL
```

## <img src="assets/icons/video.svg" width="20" /> 演示

[assets/demo.cast](assets/demo.cast) 是一段 26 秒的真实 asciinema v2 录制：置顶答案随 Agent 会话实时更新，Enter 跳转结论对应的原始 transcript 段落，tailpin diff 对比两次运行的结论差异。

```sh
asciinema play assets/demo.cast
```

TUI 的答案面板（❯ 为当前选中，每条结论下带 `行号 · uuid · 时间戳` 溯源）：

```text
tailpin · d8e9f0a1 · rev 1 · high · 20:34:21
examples/projects/-Users-dev-work-api-server/run-b.jsonl
❯ [fix]    In summary, the flaky parser test is fixed for good:
           L20 · d8e9f0a1 · 2026-09-08T10:04:40.003Z
  [finding] Root cause: CountTokens in internal/parser/parser.go double-counted the trailing newline via the i == len(s) sentinel iteration.
            L20 · d8e9f0a1 · 2026-09-08T10:04:40.003Z
  [fix]     Fix: corrected the loop boundary at parser.go:41 and added a regression case to parser_test.go.
            L20 · d8e9f0a1 · 2026-09-08T10:04:40.003Z
  [finding] Verified: go test ./... passes, 214/214.
            L20 · d8e9f0a1 · 2026-09-08T10:04:40.003Z
  [caveat]  Caveat: the config loader refactor is still pending.
            L20 · d8e9f0a1 · 2026-09-08T10:04:40.003Z
j/k claims · enter span · q quit
```

对选中的结论按 Enter，打开它对应的原始 transcript 段落（产生该结论的记录高亮，`…` 表示按终端宽度截断）：

```text
── span ───────────────────────────────────────────────────────────────────────────────────────────────────────────
claim: Root cause: CountTokens in internal/parser/parser.go double-counted the trailing newline via the i == len(s) sentinel iteration.
L20 · d8e9f0a1 · 2026-09-08T10:04:40.003Z
 12 {"type":"custom-title","customTitle":"fix flaky CountTokens test for good","sessionId":"d8e9f0a1-2222-4bbb-8ccc-000000000002"}
 13 {"type":"assistant","uuid":"d8e9f0a1-0000-4000-8000-000000000009","parentUuid":"d8e9f0a1-0000-4000-8000-000000000007","sessionId":"d8e…
 14 {"type":"assistant","uuid":"d8e9f0a1-0000-4000-8000-000000000010","parentUuid":"d8e9f0a1-0000-4000-8000-000000000009","sessionId":"d8e…
 15 {"type":"assistant","uuid":"d8e9f0a1-0000-4000-8000-000000000011","parentUuid":"d8e9f0a1-0000-4000-8000-000000000010","sessionId":"d8e…
 16 {"type":"user","uuid":"b8c1d2e3-0000-4000-8000-000000000012","parentUuid":"d8e9f0a1-0000-4000-8000-000000000011","sessionId":"d8e9f0a1…
 17 {"type":"system","parentUuid":"b8c1d2e3-0000-4000-8000-000000000012","isSidechain":false,"subtype":"stop_hook_summary","hookCount":1,"…
 18 {"type":"assistant","uuid":"d8e9f0a1-0000-4000-8000-000000000013","parentUuid":"b8c1d2e3-0000-4000-8000-000000000012","sessionId":"d8e…
 19 {"type":"last-prompt","lastPrompt":"run the full parser test suite when done","leafUuid":"d8e9f0a1-0000-4000-8000-000000000013","sessi…
 20 …rser test is fixed for good:\n\n- Root cause: CountTokens in internal/parser/parser.go double-counted the trailing newline via the i =…
j/k scroll · esc back · q quit
```

任务重跑一遍之后，`tailpin diff` 直接对比两次运行给出的不同答案：

```text
run-a -> run-b
~ changed  [finding]  Verified: go test ./... passes, 214/214.
  was:      Verified: go test ./... passes locally, 214/214.
+ added    [fix]     In summary, the flaky parser test is fixed for good:
+ added    [finding]  Root cause: CountTokens in internal/parser/parser.go double-counted the trailing newline via the i == len(s) sentinel iteration.
+ added    [fix]     Fix: corrected the loop boundary at parser.go:41 and added a regression case to parser_test.go.
- removed  [finding]  Summary of the first investigation:
- removed  [finding]  Root cause: the test shares mutable parser state with TestCountTokensBasic and only flakes when the two run back to back.
- removed  [fix]     Fix: added a warm-cache guard that skips the test when the shared state is dirty.
  1 claims unchanged
```

## <img src="assets/icons/bulb.svg" width="20" /> 为什么需要 tailpin

在动辄上万行的 Claude Code 长会话里，Agent 的最新结论永远埋在 transcript 尾部。你想知道"它现在到底认为答案是什么"，只能去翻一条不断变长、还夹着大量工具输出的日志；本地长思考模型上的 Opencode 会话更是把结论埋进长长的思考流里。跑一次四十分钟的任务，你真正想读的可能只有最后十行。

tailpin 把这十行变成一块**实时更新的置顶答案面板**：

- 结论按 claim 拆分，逐条标注 finding / plan / fix / caveat；
- 每条 claim 都能一键跳回产生它的原始 transcript 记录（行号、消息 uuid、时间戳）；
- 任务重跑后 `tailpin diff` 逐条对比两次运行的结论：哪些新提出、哪些被推翻、哪些改了措辞。

它和 prompt skill 是互补而不是竞争：caveman、i-have-adhd 这类 skill 改变 Agent *怎么说*，tailpin 改变你*看哪里*。tailpin 只读取磁盘上已有的 transcript，不干预生成，也不向 Agent 会话回注任何内容。

## <img src="assets/icons/architecture.svg" width="20" /> 工作原理

一个 Go 二进制，四个内部包，零模型调用：

```text
~/.claude/projects/*/*.jsonl    （只读；嵌套的 subagent / journal 文件不参与发现）
      |  追加轮询，250ms
      v
transcript adapter    解析 JSONL 方言，抽出 assistant 文本块（非消息记录按类型跳过）
      v
pin extractor         确定性打分选答案：时近性 + 结论标记（中英文）+ 代码块 + 路径；
                      降权提问、权限请求与 narration
      v
pin store             原子写 ~/.tailpin/sessions/<session_id>.pin.json
      v
TUI (bubbletea)       答案面板 / 溯源视图           diff（CLI 输出）
```

置顶答案是一个独立、可寻址、带版本的工件：

```text
PinnedAnswer { session_id, revision, updated_at, confidence, claims[] }
Claim        { text, kind: finding|plan|fix|caveat,
               span { file, line_start..line_end, byte_start..byte_end, message_uuid, timestamp } }
```

三个刻意的设计决定：

- **严格只读。** tailpin 永远不写 harness 的 transcript；它写的唯一文件是自己的 pin 工件，且用临时文件 + rename 原子落盘，中途被杀也不会留下半个 JSON。
- **确定性、全离线。** 抽结论靠启发式打分而不是 LLM——无 API key、无网络请求。启发式会看走眼，所以每条结论都带溯源：选错了一眼就能看出来，transcript 始终是 ground truth。
- **方言隔离。** 只有 adapter 认识 Claude-Code 风格 JSONL 这一种方言（消息记录、uuid、ISO 8601 时间戳；harness 混入的 mode、queue-operation、file-history-snapshot 等记录按类型跳过）。支持别的 transcript 格式意味着写一个新 adapter，而不是到处改代码。

## <img src="assets/icons/terminal-2.svg" width="20" /> 使用

| 命令 | 作用 |
| --- | --- |
| `tailpin` | 自动发现最新活跃会话，进入 TUI |
| `tailpin watch <file>` | watch 指定的会话文件（TUI） |
| `tailpin watch <file> --plain` | 无 TUI 的纯文本实时面板（适合管道 / 无交互终端） |
| `tailpin diff` | 最新会话 vs 同项目的上一个会话 |
| `tailpin diff <idA> <idB>` | 显式指定两个会话对比 |
| `tailpin help` / `tailpin version` | 帮助 / 版本 |

按键：`j`/`k` 在结论间移动（溯源视图打开时滚动视图）· `Enter` 打开当前结论的原始记录 · `esc` 返回 · `q` 退出。

行为细节：

- 会话文件追加后约 1 秒内刷新（250ms 轮询）；答案文本变化时 revision 加一，工件落盘到 `~/.tailpin/sessions/<session_id>.pin.json`，重启 tailpin 后 revision 从磁盘继续累计。
- diff 优先读已存的 pin 工件；没有 watch 过的历史会话也可以直接 diff（现场抽取）。
- `CLAUDE_CONFIG_DIR` 环境变量覆盖 harness 配置目录（默认 `~/.claude`），projects 根目录随之变为 `<dir>/projects`。

v0.1 的边界（刻意为之）：

- 只读观察。mid-run 的 accept / annotate 是 v0.2 的候选，前提是先验证真的有人把 tailpin 挂在长会话旁边用。
- 只支持 Claude-Code 风格 JSONL 一种 transcript 方言。
- 一个 tailpin 进程看一个会话；没有 Web UI、没有推送通知、不做记忆回注。
- Windows 二进制随 release 一起构建，但在 Windows 上未经测试。

## <img src="assets/icons/route.svg" width="20" /> 路线图

- **v0.1（当前）**：置顶答案、逐条 claim 溯源、双栏 TUI、跨会话 claim 级 diff。
- **v0.2（计划，视使用证据启动）**：mid-run accept / annotate——把"接受某个结论"变成一次 pin 工件的元数据编辑，而不是协议变更。
- **明确不做**：LLM 抽取（离线确定性是硬要求）、Web UI / 托管面板、多会话仪表盘、向 Agent 会话回注上下文、transcript 格式无限扩张。

## <img src="assets/icons/license.svg" width="20" /> 许可证

MIT · Copyright (c) 2026 SuperMarioYL
