# AGENTS.md - qtcloud-hr

Skills provide specialized instructions and workflows for specific tasks.
Use the skill tool to load a skill when a task matches its description.

## 文档地图

| 文档 | 认知角色 | 内容概要 |
|------|----------|----------|
| [README.md](README.md) | 陈述记忆 | 项目定位、产品模块、文档目录 |
| [CONTRIBUTING.md](CONTRIBUTING.md) | 程序记忆 | 文档构建、提交流程、工作方式 |
| [docs/index.md](docs/index.md) | 架构 | 产品简介（场景-困境-救援） |

## 文档链路

本仓库遵循五层文档体系：

```
BRD → PRD → ADD → 代码 → QA
```

每层文档回答不同问题：
- **BRD**：为什么做（业务痛点）
- **PRD**：做什么（产品需求）
- **ADD**：怎么做（架构设计）

三层文档分别在 `docs/brd/`、`docs/prd/`、`docs/add/` 目录下。

## 子模块

本仓库是主仓库 quanttide-platform 的子模块，提交需两段式：

1. 在 qtcloud-hr 内提交推送
2. 在主仓库更新子模块引用并提交推送
