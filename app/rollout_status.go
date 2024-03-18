package app

type RolloutStatus string

const (
	RolloutStatusComplete   RolloutStatus = "complete"
	RolloutStatusInProgress RolloutStatus = "in-progress"
	RolloutStatusFailed     RolloutStatus = "failed"
	RolloutStatusCancelled  RolloutStatus = "cancelled"
	RolloutStatusUnknown    RolloutStatus = "unknown"
)
