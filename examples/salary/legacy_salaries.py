"""工资"""

def calculate_salary(base_hours, hourly_rate, overtime_hours=0, deductions=0):
    """计算计时工资

    :param base_hours: 基础工时
    :param hourly_rate: 小时费率
    :param overtime_hours: 加班工时
    :param deductions: 扣款
    :return: 净工资
    """
    if any(val < 0 for val in [base_hours, hourly_rate, overtime_hours, deductions]):
        raise ValueError("所有参数必须为非负数")

    base_salary = base_hours * hourly_rate
    overtime_pay = overtime_hours * hourly_rate * 1.5
    performance_bonus = base_salary * 0.1
    net_salary = base_salary + overtime_pay + performance_bonus - deductions
    return max(net_salary, 0)
