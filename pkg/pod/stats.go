package pod

import "time"

type Stats struct {
	Created         time.Time
	Scheduled       time.Time
	Initialized     time.Time
	ContainersReady time.Time
	Ready           time.Time
}

func (s Stats) TimeToScheduled() time.Duration {
	return s.Scheduled.Sub(s.Created)
}

func (s Stats) TimeToInitialized() time.Duration {
	return s.Initialized.Sub(s.Created)
}

func (s Stats) TimeToReady() time.Duration {
	return s.Ready.Sub(s.Created)
}
