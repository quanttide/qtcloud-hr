# Contributing

## 文档构建

本文档站使用 [MyST Markdown](https://mystmd.org) 构建。

### 本地预览

```bash
pip install mystmd
cd docs
myst build --html    # 构建静态站点
myst start           # 启动本地预览服务器（默认 http://localhost:3000）
```

### 目录结构

```
docs/
├── index.md        # 产品简介
├── myst.yml        # MyST 站点配置
├── brd/            # 业务需求文档
├── prd/            # 产品需求文档
└── add/            # 架构设计文档
```

### 提交流程

1. 编辑文档后，本地 `myst build --html` 验证无报错
2. 提交推送，GitHub Actions 自动部署到 GitHub Pages
3. 站点地址：`https://quanttide.github.io/qtcloud-hr/`
