package main

import "time"

type jobResponseEnvelope struct {
	Count int   `json:"count"`
	Jobs  []job `json:"value"`
}

type job struct {
	RequestID   int       `json:"requestId"`
	Name        string    `json:"name"`
	QueueTime   time.Time `json:"queueTime"`
	AssignTime  time.Time `json:"assignTime"`
	ReceiveTime time.Time `json:"receiveTime"`
	JobID       string    `json:"jobId"`
	PlanType    string    `json:"planType"`
}
