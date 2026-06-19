// Package dealtasks holds the request DTOs for the deal-task endpoints. The response
// is the service's contracts.TaskView (already JSON-tagged).
package dealtasks

import (
	"time"

	tcontracts "github.com/romerito007/chat-smsnet-omnichannel/domain/dealtasks/contracts"
)

// CreateTaskRequest is the body of POST /v1/deals/{id}/tasks.
type CreateTaskRequest struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	DueDate     *time.Time `json:"due_date"`
	AssignedTo  string     `json:"assigned_to"`
}

// ToCommand maps the request to the service command (deal id from the path).
func (r CreateTaskRequest) ToCommand(dealID string) tcontracts.CreateTask {
	return tcontracts.CreateTask{
		DealID: dealID, Title: r.Title, Description: r.Description,
		DueDate: r.DueDate, AssignedTo: r.AssignedTo,
	}
}

// UpdateTaskRequest is the body of PATCH /v1/deals/{id}/tasks/{taskId}. Nil =
// unchanged; clear_due_date removes the due date.
type UpdateTaskRequest struct {
	Title        *string    `json:"title"`
	Description  *string    `json:"description"`
	DueDate      *time.Time `json:"due_date"`
	ClearDueDate bool       `json:"clear_due_date"`
	AssignedTo   *string    `json:"assigned_to"`
}

// ToCommand maps to the service command.
func (r UpdateTaskRequest) ToCommand() tcontracts.UpdateTask {
	return tcontracts.UpdateTask{
		Title: r.Title, Description: r.Description, DueDate: r.DueDate,
		ClearDueDate: r.ClearDueDate, AssignedTo: r.AssignedTo,
	}
}
