# 薪资核算

## 1. 设计目标

PRD 中的业务规则（加班倍数 1.5、绩效比例 10%）当前硬编码在计算函数中。设计要求是：**新增或调整薪资项时，不改引擎，只增规则**。

## 2. 领域模型

```
SalaryCalculationParams          SalaryItem
┌─────────────────────┐          ┌──────────────────┐
│ base_hours          │          │ name: str        │
│ hourly_rate         │          │ amount: Decimal  │
│ overtime_hours      │  1..n    │ category: str    │
│ deductions          │◄─────────│ (base/ overtime/ │
└─────────────────────┘          │  bonus/deduction)│
                                 └──────────────────┘
                                        ▲
                                        │ implements
                                        │
         SalaryEngine ──uses──► SalaryRule (接口)
              │                      │
              │              ┌───────┼──────────┐
              │              │       │          │
         SalaryResult   BaseRule OvertimeRule BonusRule
         ┌──────────┐
         │ items[]  │
         │ net_total│
         └──────────┘
```

## 3. 核心结构

### 3.1 SalaryRule（接口）

每个薪资项实现该接口：

```
method apply(params, context) → SalaryItem
```

- `params`：计算参数（工时、费率、扣款等）
- `context`：共享上下文，规则间可传递中间结果（如基础薪资供奖金规则引用）
- `SalaryItem`：单项结果（名称、金额、类别）

### 3.2 SalaryEngine

```
rules: list[SalaryRule]

method calculate(params) → SalaryResult
  1. 校验参数
  2. 逐条执行 rules，结果写入 context
  3. 汇总 SalaryItem → SalaryResult（含净薪资保底）
```

### 3.3 规则注册

规则列表外部注入，引擎不感知具体规则：

```
rules = [
    BaseSalaryRule(),
    OvertimeRule(multiplier=1.5),        ← 倍数是参数
    PerformanceBonusRule(ratio=0.1),      ← 比例是参数
    DeductionRule(),
]
engine = SalaryEngine(rules)
```

## 4. 扩展方式

新增一项"餐补"，只需：

1. 实现 `MealAllowanceRule(amount_per_day)`，从 context 读取工作天数
2. 注册到规则列表
3. 引擎和其余规则不变

## 5. 与 PRD 的映射

| PRD 业务规则 | ADD 对应规则 | 参数化 |
|-------------|-------------|--------|
| 基础薪资 = 工时 × 费率 | `BaseSalaryRule` | 无参数 |
| 加班薪资 = 工时 × 费率 × 1.5 | `OvertimeRule(multiplier)` | `multiplier` |
| 绩效奖金 = 基础薪资 × 10% | `PerformanceBonusRule(ratio)` | `ratio` |
| 扣除金额 | `DeductionRule` | 无参数 |
