package tracker

type event string

const (
	started   event = "started"
	regular         = ""
	completed       = "completed"
	stopped         = "stopped"
)
