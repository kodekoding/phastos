package examples

import (
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/kodekoding/phastos/v2/go/api"
)

// This file demonstrates how to use SSE (Server-Sent Events) in your application

// Example 1: Broadcasting from a controller
type NotificationController struct {
	app interface {
		BroadcastSSE(event, data string)
	}
}

func (n *NotificationController) SendNotification(ctx api.Context) error {
	// Parse request
	var req struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	}

	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(400).JSON(map[string]string{"error": "Invalid request"})
	}

	// Broadcast the notification to all SSE clients
	data, _ := json.Marshal(map[string]string{
		"message":   req.Message,
		"type":      req.Type,
		"timestamp": time.Now().Format(time.RFC3339),
	})

	n.app.BroadcastSSE("notification", string(data))

	return ctx.JSON(map[string]string{"status": "notification sent"})
}

// Example 2: Broadcasting from a use case (e.g., after employee update)
type EmployeeUsecaseWithSSE struct {
	// ... existing fields
	app interface {
		BroadcastSSE(event, data string)
	}
}

func (e *EmployeeUsecaseWithSSE) UpdateEmployee(employeeID string, data map[string]interface{}) error {
	// ... perform update logic

	// Broadcast employee update event
	eventData, _ := json.Marshal(map[string]interface{}{
		"employee_id": employeeID,
		"action":      "updated",
		"timestamp":   time.Now().Unix(),
	})

	e.app.BroadcastSSE("employee-update", string(eventData))

	return nil
}

// Example 3: Broadcasting system events
func BroadcastSystemEvent(app interface{ BroadcastSSE(event, data string) }, eventType, message string) {
	eventData := fmt.Sprintf(`{"type":"%s","message":"%s","timestamp":%d}`,
		eventType, message, time.Now().Unix())

	app.BroadcastSSE("system", eventData)
}

// Example 4: Client-side JavaScript to connect to SSE
/*
// In your frontend JavaScript:

const eventSource = new EventSource('/sse');

// Listen for connection
eventSource.onopen = () => {
    console.log('Connected to SSE');
};

// Listen for all messages (default handler)
eventSource.onmessage = (event) => {
    console.log('Received:', event.data);
};

// Listen for specific event types
eventSource.addEventListener('notification', (event) => {
    const data = JSON.parse(event.data);
    console.log('Notification:', data.message);
    // Update UI with notification
});

eventSource.addEventListener('employee-update', (event) => {
    const data = JSON.parse(event.data);
    console.log('Employee updated:', data.employee_id);
    // Refresh employee list or update specific employee
});

eventSource.addEventListener('system', (event) => {
    const data = JSON.parse(event.data);
    console.log('System event:', data.message);
    // Show system alert
});

// Error handling
eventSource.onerror = (error) => {
    console.error('SSE error:', error);
    // Will automatically reconnect
};

// Close connection when needed
// eventSource.close();
*/

// Example 5: Integration with existing app structure
func IntegrateSSEIntoApp() {
	/*
		// In your loader/app.go, the app already has BroadcastSSE method:

		func (a *app) BroadcastSSE(event, data string) {
			if a.sseHub != nil {
				a.sseHub.Broadcast(event, data)
			}
		}

		// To use it in your controllers, pass the app instance:

		// In loader/modules.go:
		employeeController := employee3.New(employeeUsecase, a) // Pass app instance

		// In your controller:
		type EmployeeController struct {
			usecase EmployeeUsecase
			app     interface{ BroadcastSSE(event, data string) }
		}

		func (e *EmployeeController) CreateEmployee(ctx api.Context) error {
			// ... create employee logic

			// Broadcast event
			e.app.BroadcastSSE("employee-created", fmt.Sprintf(`{"id":"%s"}`, newEmployeeID))

			return ctx.JSON(employee)
		}
	*/
}
