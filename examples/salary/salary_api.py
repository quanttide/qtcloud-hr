from fastapi import APIRouter, Depends, Query, HTTPException
from sqlmodel import Session
from datetime import date
from app.models.salary import SalaryCalculation, SalaryCalculationCreate, SalaryCalculationRead
from app.schemas.salary import SalaryResult, SalaryCalculationParams
from app.services import salary_calculation as service
from app.database import get_session

router = APIRouter()

@router.post("/calculate", response_model=SalaryResult)
def calculate_salary(params: SalaryCalculationParams):
    return service.calculate_salary(params)

@router.post("/records", response_model=SalaryCalculationRead)
def create_salary_record(
    record: SalaryCalculationCreate,
    session: Session = Depends(get_session)
):
    return service.create_salary_record(session, record)

@router.get("/records/{record_id}", response_model=SalaryCalculationRead)
def get_record_by_id(
    record_id: int,
    session: Session = Depends(get_session)
):
    record = session.get(SalaryCalculation, record_id)
    if not record:
        raise HTTPException(status_code=404, detail="薪资记录不存在")
    return record

@router.get("/records", response_model=list[SalaryCalculationRead])
def get_records_by_period(
    period_start: date = Query(..., description="薪资周期开始日期"),
    period_end: date = Query(..., description="薪资周期结束日期"),
    department: str = Query(None, description="可选: 按部门筛选"),
    session: Session = Depends(get_session)
):
    return service.get_records_by_period(session, period_start, period_end, department)


@router.get("/records/by-employee/{employee_id}", response_model=list[SalaryCalculationRead])
def get_records_by_employee(
        employee_id: int,
        session: Session = Depends(get_session)
):
    query = session.query(SalaryCalculation).filter(
        SalaryCalculation.employee_id == employee_id
    )
    records = query.order_by(SalaryCalculation.period_start.desc()).all()

    if not records:
        raise HTTPException(
            status_code=404,
            detail=f"未找到员工ID {employee_id} 的薪资记录"
        )

    return records

@router.delete("/records/{record_id}", status_code=204)
def delete_salary_record(
        record_id: int,
        session: Session = Depends(get_session)
):
    record = session.get(SalaryCalculation, record_id)
    if not record:
        raise HTTPException(status_code=404, detail="薪资记录不存在")

    session.delete(record)
    session.commit()
    return
