"""工资计算单元测试"""
import pytest


def test_basic_salary_calculation():
    from qtadmin_provider.salaries import calculate_salary

    assert calculate_salary(160, 100) == 160*100 + 160*100*0.1
    assert calculate_salary(160, 100, 10) == (160*100) + (10*100*1.5) + (160*100*0.1)
    assert calculate_salary(160, 100, deductions=500) == (160*100*1.1) - 500


def test_boundary_conditions():
    from qtadmin_provider.salaries import calculate_salary

    assert calculate_salary(0, 100, deductions=1000) == 0
    assert calculate_salary(175, 80, 0) == 175*80*1.1


def test_invalid_inputs():
    from qtadmin_provider.salaries import calculate_salary

    with pytest.raises(ValueError):
        calculate_salary(-40, 100)

    with pytest.raises(ValueError):
        calculate_salary(160, -20)
