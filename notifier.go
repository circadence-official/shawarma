package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	activeStatus   = "active"
	inactiveStatus = "inactive"

	retryAttempts = 3
)

type stateChangeDto struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

var retryInterval, _ = time.ParseDuration("1s")

func notifyStateChange(info *monitorInfo, newStatus bool) error {
	var err error

	state := stateChangeDto{
		Service: info.Name,
	}

	if newStatus {
		state.Status = activeStatus
	} else {
		state.Status = inactiveStatus
	}

	body, err := json.Marshal(&state)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, info.URL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	for i := 0; i < retryAttempts; i++ {
		client := &http.Client{}
		resp, err := client.Do(req)
		if resp != nil {

			defer resp.Body.Close()

			log.WithFields(log.Fields{
				"label": info.Name,
				"pod": info.PodName,
				"ns":  info.Namespace,
			}).Debug("Notification result ", resp.Status)

			if err == nil {
				return nil
			}
		}

		time.Sleep(retryInterval)
	}

	return err
}
