package pod

import "time"

type EventType string

const (
	Created         EventType = "Created"
	Scheduled       EventType = "Scheduled"
	Initialized     EventType = "Initialized"
	ContainersReady EventType = "ContainersReady"
	Ready           EventType = "Ready"
	Deleted         EventType = "Deleted"
)

type Event struct {
	Name string
	Type EventType
	Time time.Time
}
