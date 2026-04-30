import pytest

from app.schemas.salary import SalaryCalculationParams
from app.services.salary_calculation import calculate_salary


def test_calculate_salary_valid_params():
    params = SalaryCalculationParams(
        base_hours=160,
        hourly_rate=25,
        overtime_hours=10,
        deductions=200
    )

    result = calculate_salary(params)

    base_salary = 160 * 25
    overtime_pay = 10 * 25 * 1.5
    performance_bonus = base_salary * 0.1
    net_salary = base_salary + overtime_pay + performance_bonus - 200

    assert result.net_salary == net_salary
    assert result.base_salary == base_salary
    assert result.overtime_pay == overtime_pay
    assert result.performance_bonus == performance_bonus


def test_calculate_salary_negative_params():
    from pydantic import ValidationError
    with pytest.raises(ValidationError) as exc_info:
        SalaryCalculationParams(
            base_hours=-10,
            hourly_rate=25,
            overtime_hours=0,
            deductions=0
        )
    assert "ensure this value is greater than or equal to 0" in str(exc_info.value)
