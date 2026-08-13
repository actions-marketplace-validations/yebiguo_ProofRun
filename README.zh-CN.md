<div align="center">

# ProofRun

**给 AI 编程 Agent 用的本地验证凭证。**

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go 1.22+](https://img.shields.io/badge/go-1.22%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![CI](https://github.com/yebiguo/proofrun/actions/workflows/test.yml/badge.svg)](https://github.com/yebiguo/proofrun/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/yebiguo/proofrun/graph/badge.svg)](https://codecov.io/gh/yebiguo/proofrun)
[![Release](https://img.shields.io/github/v/release/yebiguo/proofrun?include_prereleases&label=release)](https://github.com/yebiguo/proofrun/releases)

[English](README.md) · [简体中文](README.zh-CN.md)

</div>

---

![ProofRun 演示：跑一个检查，PASS；改动代码；自动变成 STALE](docs/demo.gif)

ProofRun 不判断你的代码写得对不对。它只证明——用密码学指纹，而不是靠信任——哪些检查真的在你现在这份确切的代码上跑过。

## 问题在哪

AI 编程 Agent 说"所有测试都通过了"。这句话是真的吗？

也许是。它上一次真的跑测试的时候，这句话确实是真的。但那可能是三次修改之前的事了。Agent 甚至未必记得自己有没有真的跑过——它可能只是在推断"这个改动看起来没问题，测试应该还能过"。光看这句话本身，你没法分辨"我跑了，而且真的过了"和"我觉得应该会过"之间的区别。

ProofRun 补的就是这个缺口。不是靠让 Agent 更诚实，而是让这句话本身变得可以被核查。

## 怎么用

```bash
$ proofrun run test -- pytest
...
test: pass (exit 0, 1841ms)

$ proofrun status
test                 PASS    (exit 0, 1841ms)

# 这之后代码被改了——不管改的人是 Agent 还是人类

$ proofrun status
test                 STALE   (last run: pass, exit 0 — code changed since)
```

每一条检查结果都绑定着你当前代码状态的指纹：git commit，加上所有未提交改动（不管有没有 stage、有没有被 git 跟踪）算出的哈希。哪怕只改了一个字节，结果都会自动变成 `STALE`。不需要谁记得去问"这个 PASS 现在还作数吗"。

## 安装

```bash
curl -L https://github.com/yebiguo/proofrun/releases/download/v0.1.0/proofrun_linux_amd64.tar.gz | tar xz
# 其他平台见 https://github.com/yebiguo/proofrun/releases
```

或者从源码构建：

```bash
go install github.com/yebiguo/proofrun/cmd/proofrun@latest
```

## 快速上手

```bash
proofrun init                      # 生成 .proofrun.yml
proofrun run test -- pytest        # 真实执行 pytest，把结果绑定到当前代码
proofrun status --strict           # 只要有检查不是 PASS 就非零退出
```

## 为什么不能只靠信任 Agent

- **全程零 LLM 调用。** ProofRun 不用 AI 去验证 AI。它就是启动一个真实子进程，读它真实的退出码——机制就这么简单。
- **只有四种状态，没有一种是猜的。** `PASS`、`FAIL`、`STALE`、`NOT RUN`——每一个都来自一次被观察到的真实执行，或者"确实没执行过"这个事实本身。没有第五种"大概率没问题"。
- **完全离线。** 零网络请求、零遥测、零账号。
- **比对的是参数数组，不是拼接字符串。** 配置里声明的 `pytest -k "foo bar"`，不会被一条拼接后长得像、但实际参数边界不同的命令蒙混过去——ProofRun 比对的是真实的参数数组，不是拼出来的文本。

## ProofRun 刻意不做的事

不解析测试输出，不判断代码质量，不自动修复任何东西，暂时还没接入 CI（计划中的 GitHub Action 会自己独立重新验证一遍，而不是直接信任某台笔记本本地生成的 receipt——本地生成的 receipt 本来就不该被服务端直接信任）。完整边界见 [AGENTS.md](AGENTS.md)。

## 由 AI agent 写出来，被另一个 AI agent 揪出问题

ProofRun 自己的代码是由 AI 编程 agent（Claude Code）在人类主导下写出来的，发布第一个版本前经过了好几轮独立的、只读的对抗性审核。那次审核发现，ProofRun 自己的命令比对逻辑能被骗过：一个被 shell 引号意外吞并的参数，能让一条检查悄悄跑了零个测试，还照样报 `PASS`。完整复现过程、具体修法、为什么简单打个补丁不够 → [docs/case-study.md](docs/case-study.md)（英文）。

每一处修复在被接受之前，都用真实复现验证过，不是"看起来合理就通过"。一个存在的目的就是让 AI agent 对自己的话负责的工具，如果连同样的审视用在自己身上都扛不住，那它就没有存在的必要。

## 命令

```bash
proofrun init                      # 生成 .proofrun.yml
proofrun run <check-name> -- <cmd> # 真实执行 <cmd>，把退出码+耗时绑定到当前 git 状态
proofrun status [--strict]         # 每条检查的 PASS / FAIL / STALE / NOT RUN；--strict 下只要有 required 检查不是 PASS 就非零退出
proofrun report [--json]           # 完整报告，人类可读或 JSON
```

## 配置：`.proofrun.yml`

```yaml
checks:
  test:
    command: [pytest]
    required: true
  build:
    command: [npm, run, build]
    required: true
  lint:
    command: [ruff, check, .]
    required: false
```

`command` 是参数数组，不是 shell 字符串——ProofRun 从不经过 shell，比对"实际跑的"和"配置声明的"必须逐个参数精确匹配。`required: true` 是让一条检查能挡住 `status --strict` 的开关，这正是接入 pre-commit hook 或 CI 门禁时要用的字段。

## 指纹是怎么算的

每条结果都绑定着当前 git `HEAD`，加上 `git diff HEAD` 与所有未跟踪、未被 gitignore 的文件内容一起算出的 SHA-256 哈希。`proofrun status` 每次都会重新算一遍这个指纹，跟本地存的对比——哪怕只是改了一个空格、或者多了一个新文件，都会被判定为 `STALE`。

## 路线图

- **v0.2** —— 接入 GitHub Action，让 CI 自己独立重新验证，而不是直接信任某台笔记本本地生成的 receipt
- **v0.3** —— 支持解析常见测试框架（pytest、Jest、JUnit）的结构化输出
- 带签名、防篡改的 receipt 已经在考虑范围内，还没有设计方案

## 参与贡献

欢迎 Issue 和 PR。这还是一个年轻的项目（v0.1），范围收得很窄，而且是刻意收窄的——在提议任何涉及 STALE 判定逻辑或 receipt schema 的改动之前，先看一眼 [AGENTS.md](AGENTS.md)：这两块是这个项目最经不起出错的地方。

## 许可证

MIT
