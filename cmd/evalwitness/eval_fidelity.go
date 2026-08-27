package main

import "github.com/Christopher-Schulze/evalwitness/internal/preprocess"

func aggregatePreparedEvidence(tasks [][]preprocess.Result) preprocess.AccountingAggregate {
	count := 0
	for _, task := range tasks {
		count += len(task)
	}
	results := make([]preprocess.Result, 0, count)
	for _, task := range tasks {
		results = append(results, task...)
	}
	return preprocess.AggregateAccounting(results...)
}
