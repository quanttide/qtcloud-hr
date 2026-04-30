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

## 工作方式

### 逆向文档化流程

从既有代码反推产品文档，镜像正向开发的文档链路：

```
代码 → ADD（架构设计） → PRD（产品需求） → BRD（业务痛点）
```

正向开发流程相反：`BRD → PRD → ADD → 代码`。

### 实操记录

本仓库的 salary 和 recruitment 两个模块已完成逆文档化作为模板：

1. **剥离杂质**：先移除框架/基础设施依赖（FastAPI、ORM、DB），只留纯领域逻辑
2. **逆向推导**：从代码反推 ADD（领域模型/核心结构/扩展方式），再推导 PRD（用户故事/验收标准/业务规则），再推导 BRD（场景/困境/救援）
3. **叙事风格**：产品简介和 BRD 使用"场景-困境-救援"三段式，PRD 使用用户故事 + Given-When-Then 验收标准
4. **静态站点**：使用 MyST Markdown 构建文档站，GitHub Actions 自动部署到 GitHub Pages

### 注意事项

- 编写 PRD 前，先加载 `product-prd` SKILL（`/home/iguo/repos/quanttide/assets/quanttide-platform/.agents/skills/product-prd/SKILL.md`）确保格式规范
- BRD 使用 `product-brd` SKILL 标准格式（场景-工作角色-问题四要素-假设句式）
- 提交前确认子仓库和主仓库两段式提交均已推送
