package app

type RolloutStatus string

const (
	RolloutStatusComplete   RolloutStatus = "complete"
	RolloutStatusPending    RolloutStatus = "pending"
	RolloutStatusInProgress RolloutStatus = "in-progress"
	RolloutStatusFailed     RolloutStatus = "failed"
	RolloutStatusCancelled  RolloutStatus = "cancelled"
	RolloutStatusUnknown    RolloutStatus = "unknown"
)
