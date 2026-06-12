package main

import (
	"log"
	"time"
)

// startScheduler polls for instances whose scheduled start time has arrived and
// starts them, then clears the schedule. Coarse (20s) granularity is plenty for
// race start times.
func startScheduler() {
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			checkSchedules()
		}
	}()
}

func checkSchedules() {
	now := time.Now().Unix()
	for _, inst := range Instances.All() {
		if inst.Conf.ScheduledStart == nil || *inst.Conf.ScheduledStart == 0 {
			continue
		}
		if *inst.Conf.ScheduledStart > now {
			continue
		}

		if !inst.isRunning() {
			log.Printf("Scheduled start firing for instance %s", inst.Name())
			if inst.serverApplyTrack() {
				inst.start()
			} else {
				log.Printf("Scheduled start: nothing queued to run for instance %s", inst.Name())
			}
		}

		// One-shot: clear the schedule whether or not it could start.
		if _, err := Dba.updateServerInstanceSchedule(inst.Id(), nil); err != nil {
			log.Print("Could not clear scheduled start: ", err)
		}
		if err := Instances.LoadFromDb(); err != nil {
			log.Print("Could not refresh instances after scheduled start: ", err)
		}
	}
}
