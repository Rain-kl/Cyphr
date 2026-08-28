// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_inproc_cron

import (
	"context"
	"encoding/json"
	"sync"

	"Wavelet/core/extpoints"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/drivers/driver_inproc_worker"
	"github.com/robfig/cron/v3"
)

type inprocScheduler struct {
	mu          sync.RWMutex
	cronRunner  *cron.Cron
	scheduleReg extpoints.ScheduleExtension
	taskReg     extpoints.TaskExtension
	running     bool
}

func newInprocScheduler(scheduleReg extpoints.ScheduleExtension, taskReg extpoints.TaskExtension) *inprocScheduler {
	return &inprocScheduler{
		cronRunner:  cron.New(cron.WithSeconds()),
		scheduleReg: scheduleReg,
		taskReg:     taskReg,
	}
}

func (s *inprocScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return nil
	}

	if s.scheduleReg != nil {
		for _, def := range s.scheduleReg.Schedules() {
			s.registerJob(def)
		}
	}

	s.cronRunner.Start()
	s.running = true
	return nil
}

func (s *inprocScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	ctx := s.cronRunner.Stop()
	<-ctx.Done()
	s.running = false
}

func (s *inprocScheduler) registerJob(def extpoints.ScheduleDefinition) {
	spec := def.Spec
	taskType := def.TaskType

	// Support 5-field cron expression by prepending "0 " for 6-field seconds parser
	fields := len(cronFields(spec))
	cronSpec := spec
	if fields == 5 {
		cronSpec = "0 " + spec
	}

	var payloadBytes []byte
	if def.Payload != nil {
		switch p := def.Payload.(type) {
		case []byte:
			payloadBytes = p
		case string:
			payloadBytes = []byte(p)
		default:
			payloadBytes, _ = json.Marshal(p)
		}
	}

	_, err := s.cronRunner.AddFunc(cronSpec, func() {
		_, dispatchErr := driver_inproc_worker.DispatchTask(context.Background(), taskType, payloadBytes, "inproc_cron")
		if dispatchErr != nil {
			logger.ErrorF(context.Background(), "driver_inproc_cron: dispatch task %q failed: %v", taskType, dispatchErr)
		}
	})
	if err != nil {
		logger.ErrorF(context.Background(), "driver_inproc_cron: invalid cron spec %q for task %q: %v", spec, taskType, err)
	}
}

func cronFields(s string) []string {
	var fields []string
	var current []rune
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if len(current) > 0 {
				fields = append(fields, string(current))
				current = nil
			}
		} else {
			current = append(current, r)
		}
	}
	if len(current) > 0 {
		fields = append(fields, string(current))
	}
	return fields
}
