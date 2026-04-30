import pytest
from fastapi.testclient import TestClient
from sqlmodel import SQLModel, create_engine, Session
from datetime import date

from app.__main__ import app
from app.database import get_session
from app.models.employee import Employee
from app.models.salary import SalaryCalculation


@pytest.fixture(name="engine")
def engine_fixture():
    engine = create_engine(
        "sqlite:///:memory:",
        echo=False,
        connect_args={"check_same_thread": False}
    )
    SQLModel.metadata.create_all(engine)
    return engine


@pytest.fixture(name="session")
def session_fixture(engine):
    with Session(engine) as session:
        yield session


@pytest.fixture(name="client")
def client_fixture(session):
    def get_session_override():
        return session

    app.dependency_overrides[get_session] = get_session_override

    client = TestClient(app)
    yield client
    app.dependency_overrides.clear()


def test_full_salary_workflow(client, session):
    employee_payload = {"name": "李四", "position": "设计师", "department": "设计部"}
    employee_response = client.post("/api/v1/employees", json=employee_payload)
    assert employee_response.status_code == 201
    employee = employee_response.json()

    salary_params = {
        "base_hours": 160,
        "hourly_rate": 30,
        "overtime_hours": 5,
        "deductions": 150
    }
    calculate_response = client.post("/api/v1/salary/calculate", json=salary_params)
    assert calculate_response.status_code == 200

    salary_record = {
        "employee_id": employee["id"],
        "base_hours": salary_params["base_hours"],
        "hourly_rate": salary_params["hourly_rate"],
        "overtime_hours": salary_params["overtime_hours"],
        "deductions": salary_params["deductions"],
        "period_start": "2025-01-01",
        "period_end": "2025-01-31"
    }
    record_response = client.post("/api/v1/salary/records", json=salary_record)
    assert record_response.status_code == 200
    saved_record = record_response.json()

    base_salary = salary_params["base_hours"] * salary_params["hourly_rate"]
    overtime_pay = salary_params["overtime_hours"] * salary_params["hourly_rate"] * 1.5
    performance_bonus = base_salary * 0.1
    net_salary = base_salary + overtime_pay + performance_bonus - salary_params["deductions"]

    assert saved_record["calculated_salary"] == net_salary

    query_params = {
        "period_start": "2025-01-01",
        "period_end": "2025-01-31"
    }
    records_response = client.get("/api/v1/salary/records", params=query_params)
    assert records_response.status_code == 200
    records = records_response.json()

    assert len(records) == 1
    assert records[0]["id"] == saved_record["id"]
    assert records[0]["employee_id"] == employee["id"]

    dept_records = client.get(
        "/api/v1/salary/records",
        params={
            "period_start": "2025-01-01",
            "period_end": "2025-01-31",
            "department": "设计部"
        }
    )
    assert dept_records.status_code == 200
    assert len(dept_records.json()) == 1

    employee_details = client.get(f"/api/v1/employees/{employee['id']}")
    assert employee_details.status_code == 200
    assert len(employee_details.json()["salaries"]) == 1
    assert employee_details.json()["salaries"][0]["id"] == saved_record["id"]
