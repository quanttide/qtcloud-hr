"""工资计算单元测试"""
import pytest
from fastapi.testclient import TestClient
from qtadmin_provider.main import app


@pytest.fixture
def client():
    with TestClient(app) as test_client:
        yield test_client


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


def test_valid_salary_calculation(client):
    base_response = client.post("/salaries/calculate", json={
        "base_hours": 160,
        "hourly_rate": 100,
        "overtime_hours": 10,
        "deductions": 500
    })
    assert base_response.status_code == 200
    assert base_response.json()["net_salary"] == 160*100 + 10*150 - 500


def test_boundary_overtime_threshold(client):
    response = client.post("/salaries/calculate", json={
        "base_hours": 175,
        "hourly_rate": 80,
        "overtime_hours": 0,
        "deductions": 0
    })
    assert response.status_code == 200
    assert response.json()["overtime_pay"] == 0


def test_invalid_negative_hours(client):
    response = client.post("/salaries/calculate", json={
        "base_hours": -40,
        "hourly_rate": 100,
        "overtime_hours": 10
    })
    assert response.status_code == 422


def test_unsupported_salary_type(client):
    response = client.post("/salaries/calculate", json={
        "salary_type": "daily",
        "days_worked": 20,
        "daily_rate": 500
    })
    assert response.status_code == 422
