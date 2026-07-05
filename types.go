package sdk

import "context"

const (
	RuntimeBuiltin     = "builtin"
	RuntimeDeclarative = "declarative"
	RuntimeWebhook     = "webhook"
	RuntimeGoPlugin    = "go-plugin"
	RuntimeProcess     = "process"

	EventMessageReceived    = "message.received"
	EventMessageSent        = "message.sent"
	EventGroupMemberJoined  = "group.member_joined"
	EventGroupMemberLeft    = "group.member_left"
	EventGroupMembersSynced = "group.members_synced"
	EventScheduledTaskRun   = "scheduled_task.run"
	EventPaymentRedPacket   = "payment.red_packet"
	EventPaymentTransfer    = "payment.transfer"
	EventContactAdded       = "contact.added"

	ActionSendMessage  = "send_message"
	ActionAddFriend    = "add_friend"
	ActionUpdateRemark = "update_remark"
	ActionSetLabel     = "set_label"
	ActionCreateTask   = "create_task"

	PermissionMessageRead  = "message:read"
	PermissionMessageSend  = "message:send"
	PermissionAccountRead  = "account:read"
	PermissionContactRead  = "contact:read"
	PermissionContactWrite = "contact:write"
	PermissionTaskWrite    = "task:write"
)

type Permission string

type SettingsSchema struct {
	Type       string         `json:"type,omitempty"`
	Required   []string       `json:"required,omitempty"`
	Properties map[string]any `json:"properties,omitempty"`
}

type Manifest struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Version        string         `json:"version"`
	Runtime        string         `json:"runtime,omitempty"`
	Category       string         `json:"category,omitempty"`
	Summary        string         `json:"summary,omitempty"`
	Description    string         `json:"description"`
	Events         []string       `json:"events,omitempty"`
	Permissions    []Permission   `json:"permissions,omitempty"`
	SettingsSchema SettingsSchema `json:"settingsSchema,omitempty"`
	Icon           string         `json:"icon,omitempty"`
	Previews       []string       `json:"previews,omitempty"`
}

type Event struct {
	Type        string         `json:"type"`
	UserID      int            `json:"userId,omitempty"`
	AccountWxid string         `json:"accountWxid,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	OccurredAt  int64          `json:"occurredAt"`
}

type Action struct {
	ID          string         `json:"id,omitempty"`
	Type        string         `json:"type"`
	AccountWxid string         `json:"accountWxid,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type ActionResult struct {
	PluginID string         `json:"pluginId"`
	ActionID string         `json:"actionId,omitempty"`
	Type     string         `json:"type"`
	Success  bool           `json:"success"`
	Message  string         `json:"message,omitempty"`
	Payload  map[string]any `json:"payload,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Error    string         `json:"error,omitempty"`
}

type Plugin interface {
	Manifest() Manifest
	OnEvent(context.Context, Event) ([]Action, error)
	OnActionResult(context.Context, ActionResult) error
}
