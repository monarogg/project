package elevator_control

import (
	"time"
)

// starter/nullstiller en timer til et nytt antall sekunder
func RestartTimer(timer *time.Timer, sec int) {
	timer.Reset(time.Duration(sec) * time.Second)
}

// stopper en aktiv timer
func KillTimer(timer *time.Timer) {
	if !timer.Stop() {
		<-timer.C
	}
}