from sqlmodel import select
from fastapi import HTTPException

from app.models.salary import SalaryCalculation, SalaryCalculationCreate
from app.models.employee import Employee
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


def create_salary_record(session, record_data: SalaryCalculationCreate):
    calc_params = SalaryCalculationParams(
        base_hours=record_data.base_hours,
        hourly_rate=record_data.hourly_rate,
        overtime_hours=record_data.overtime_hours,
        deductions=record_data.deductions
    )

    salary_result = calculate_salary(calc_params)

    db_record = SalaryCalculation(
        employee_id=record_data.employee_id,
        base_hours=record_data.base_hours,
        hourly_rate=record_data.hourly_rate,
        overtime_hours=record_data.overtime_hours,
        deductions=record_data.deductions,
        period_start=record_data.period_start,
        period_end=record_data.period_end,
        calculated_salary=salary_result.net_salary
    )

    session.add(db_record)
    session.commit()
    session.refresh(db_record)
    return db_record


def get_records_by_period(session, period_start, period_end, department=None):
    query = select(SalaryCalculation).where(
        SalaryCalculation.period_start >= period_start,
        SalaryCalculation.period_end <= period_end
    )

    if department:
        query = query.join(Employee).where(Employee.department == department)

    return session.exec(query).all()
