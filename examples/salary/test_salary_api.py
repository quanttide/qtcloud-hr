import pytest
from fastapi.testclient import TestClient
from sqlmodel import SQLModel, create_engine, Session
from datetime import date

from app.__main__ import app
from app.database import get_session
from app.models.salary import SalaryCalculation
from app.models.employee import Employee


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


@pytest.fixture(name="test_employee")
def create_test_employee(session):
    employee = Employee(name="测试员工", position="工程师", department="技术部")
    session.add(employee)
    session.commit()
    session.refresh(employee)
    return employee


def test_calculate_salary(client):
    payload = {
        "base_hours": 160,
        "hourly_rate": 25,
        "overtime_hours": 10,
        "deductions": 200
    }

    response = client.post("/api/v1/salary/calculate", json=payload)
    assert response.status_code == 200
    result = response.json()

    base_salary = 160 * 25
    overtime_pay = 10 * 25 * 1.5
    performance_bonus = base_salary * 0.1
    net_salary = max((base_salary + overtime_pay + performance_bonus - 200), 0)

    assert result["net_salary"] == net_salary
    assert result["base_salary"] == base_salary
    assert result["overtime_pay"] == overtime_pay
    assert result["performance_bonus"] == performance_bonus


def test_create_salary_record(client, session, test_employee):
    payload = {
        "employee_id": test_employee.id,
        "base_hours": 160,
        "hourly_rate": 25,
        "overtime_hours": 10,
        "deductions": 200,
        "period_start": "2025-01-01",
        "period_end": "2025-01-31"
    }

    response = client.post("/api/v1/salary/records", json=payload)
    assert response.status_code == 200

    record = response.json()
    assert record["employee_id"] == test_employee.id
    assert record["calculated_salary"] > 0
    assert "id" in record


def test_get_salary_records(client, session, test_employee):
    record = SalaryCalculation(
        employee_id=test_employee.id,
        base_hours=160,
        hourly_rate=25,
        overtime_hours=10,
        deductions=200,
        period_start=date(2025, 1, 1),
        period_end=date(2025, 1, 31),
        calculated_salary=5000
    )
    session.add(record)
    session.commit()

    response = client.get(
        "/api/v1/salary/records",
        params={
            "period_start": "2025-01-01",
            "period_end": "2025-01-31"
        }
    )

    assert response.status_code == 200
    records = response.json()
    assert len(records) == 1
    assert records[0]["employee_id"] == test_employee.id

    response = client.get(
        "/api/v1/salary/records",
        params={
            "period_start": "2025-01-01",
            "period_end": "2025-01-31",
            "department": "技术部"
        }
    )
    assert response.status_code == 200
    assert len(response.json()) == 1

    response = client.get(
        "/api/v1/salary/records",
        params={
            "period_start": "2025-01-01",
            "period_end": "2025-01-31",
            "department": "市场部"
        }
    )
    assert response.status_code == 200
    assert len(response.json()) == 0


def test_invalid_salary_calculation(client):
    invalid_payload = {
        "base_hours": -10,
        "hourly_rate": 25
    }
    response = client.post("/api/v1/salary/calculate", json=invalid_payload)
    assert response.status_code == 422

    missing_payload = {"base_hours": 160}
    response = client.post("/api/v1/salary/calculate", json=missing_payload)
    assert response.status_code == 422
