package main

import (
	"context"
	"log"
	"net/http"
	"time"
)

func (opt *Opt) runWaitFor(ctx context.Context, client *http.Client) (string, int) {
	consecutive := opt.Consecutive - 1
	requestNum := 0
	for ctx.Err() == nil {
		requestNum++
		okMsg, errReq := opt.Request(ctx, client)
		interval := opt.Interim
		if errReq == nil && consecutive <= 0 {
			log.Printf("request[%d]: %s", requestNum, okMsg)
			return okMsg, OK
		} else if errReq == nil {
			consecutive--
			log.Printf("request[%d]: %s", requestNum, okMsg)
		} else {
			interval = opt.WaitForInterval
			consecutive = opt.Consecutive - 1
			log.Printf("request[%d]: %s", requestNum, errReq.Error())
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
	}
	return "Give up waiting for success", UNKNOWN
}

func (opt *Opt) runRequest(ctx context.Context, client *http.Client) (string, int) {
	consecutive := opt.Consecutive - 1
	requestNum := 0
	var rErr *RequestError
	for ctx.Err() == nil {
		var okMsg string
		requestNum++
		okMsg, rErr = opt.Request(ctx, client)
		if rErr == nil && consecutive <= 0 {
			log.Printf("request[%d]: %s", requestNum, okMsg)
			return okMsg, OK
		} else if rErr == nil {
			consecutive--
			log.Printf("request[%d]: %s", requestNum, okMsg)
		} else {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(opt.Interim):
		}
	}
	if rErr == nil {
		return "HTTP UNKNOWN - timeout", UNKNOWN
	}
	return rErr.Error(), rErr.Code()
}
