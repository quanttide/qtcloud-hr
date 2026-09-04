// Package domain 工时领域模型。
package domain

import "time"

// Timesheet 工时记录：员工在某日某任务投入的工时。
type Timesheet struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`    // 员工（账号系统用户 ID）
	Date      string    `json:"date"`      // 日期（YYYY-MM-DD）
	Hours     float64   `json:"hours"`     // 工时（小时）
	Project   string    `json:"project"`   // 项目/任务
	Category  string    `json:"category"`  // 分类（研发/教学/管理等）
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
