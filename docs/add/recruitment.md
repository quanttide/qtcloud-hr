# 招聘

## 1. 设计目标

PRD 中的核心需求是：AI 基于招聘方配置的筛选标准自动执行初筛，并通过反馈持续优化。设计要求是：**筛选标准可配置、AI 决策过程可解释、反馈可闭环**。

## 2. 领域模型

```
JobPosition                         Resume
┌──────────────────────┐            ┌──────────────────────┐
│ id: str              │            │ id: str              │
│ title: str           │     1:n    │ name: str            │
│ description: str     │◄───────────│ parsed_skills: []    │
│ screening_rules: []  │            │ experience_years: int│
└──────────────────────┘            │ education: str       │
                                    │ history: []          │
         ScreeningEngine            └──────────────────────┘
         ┌──────────────────┐              ▲
         │ rules: Rule[]    │              │ input
         │ matcher: AI      │──────────────┘
         │ feedbacks: []    │
         └──────────────────┘
                  │
                  ▼
         ScreeningResult
         ┌──────────────────┐
         │ resume_id: str   │
         │ decision: enum   │  (pass / reject / priority)
         │ reasons: []      │
         │ confidence: float│
         └──────────────────┘
                  ▲
                  │ corrected by
                  │
         Feedback
         ┌──────────────────┐
         │ result_id: str   │
         │ correct_decision │
         │ reason: str      │
         └──────────────────┘
```

## 3. 核心结构

### 3.1 ScreeningRule（标准接口）

```
method evaluate(resume, job) → RuleResult
  返回: {matched: bool, reason: str, weight: float}
```

每条规则独立判断，互不感知。

### 3.2 ScreeningEngine

```
rules: list[ScreeningRule]
ai_matcher: AIMatcher

method screen(resume, job) → ScreeningResult
  1. 逐条执行 rules（硬性条件）
  2. 任一不满足 → reject，记录原因
  3. 全部满足 → ai_matcher 评估加分项和匹配度
  4. 输出结果（pass / priority）及置信度
```

### 3.3 规则体系

| 规则 | 类型 | 说明 |
|------|------|------|
| `HardRequirementRule` | 硬性 | 学历、年限、必备技能，一票否决 |
| `BonusRule` | 加分 | 加分项，满足则提升优先级 |
| `AIMatcher` | 语义 | 基于 LLM 的语义匹配，处理技能名称表述差异 |

### 3.4 反馈闭环

```
HR 纠正结果
    │
    ▼
生成 Feedback（含正确判断 + 原因）
    │
    ▼
反馈写入 FeedbackStore
    │
    ▼
ScreeningEngine 下次初始化时加载反馈
    调整规则参数或 prompt
```

## 4. 扩展方式

新增筛选维度（如"期望薪资是否在预算范围内"）：

1. 实现 `SalaryMatchRule`
2. 注册到规则的规则列表
3. 引擎和其余规则不变

## 5. 与 PRD 的映射

| PRD 用户故事 | ADD 对应组件 |
|-------------|-------------|
| 故事 1：AI 自动初筛 | `ScreeningEngine` + `AIMatcher` |
| 故事 2：配置筛选标准 | `HardRequirementRule` + `BonusRule` |
| 故事 3：审核与反馈 | `Feedback` + 反馈闭环机制 |
