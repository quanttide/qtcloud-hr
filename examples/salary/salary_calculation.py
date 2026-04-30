from app.schemas.salary import SalaryResult, SalaryCalculationParams


def calculate_salary(params: SalaryCalculationParams) -> SalaryResult:
    base_salary = params.base_hours * params.hourly_rate
    overtime_pay = params.overtime_hours * params.hourly_rate * 1.5
    performance_bonus = base_salary * 0.1
    net_salary = base_salary + overtime_pay + performance_bonus - params.deductions

    return SalaryResult(
        base_salary=round(base_salary, 2),
        overtime_pay=round(overtime_pay, 2),
        performance_bonus=round(performance_bonus, 2),
        net_salary=round(max(net_salary, 0), 2),
        deduction=params.deductions
    )
