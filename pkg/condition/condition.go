package condition

import (
	"errors"
	"time"
)

type Condition struct {
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	LastUpdateTime     time.Time `json:"lastUpdateTime"`
	Message            string    `json:"message,omitempty"`
	Status             string    `json:"status"`
	Type               string    `json:"type"`
}

func GetLatestUpdateCondition(slice []Condition) []Condition {
	ret := map[time.Time][]Condition{}
	var last time.Time
	initialized := false
	for _, c := range slice {
		if !initialized {
			last = c.LastUpdateTime
			initialized = true
		}
		if last.Before(c.LastUpdateTime) {
			last = c.LastUpdateTime
		}
		if _, ok := ret[c.LastUpdateTime]; !ok {
			ret[c.LastUpdateTime] = []Condition{}
		}
		ret[c.LastUpdateTime] = append(ret[c.LastUpdateTime], c)

	}
	return ret[last]
}

func GetLatestTransitionCondition(slice []Condition) []Condition {
	ret := map[time.Time][]Condition{}
	var last time.Time
	initialized := false
	for _, c := range slice {
		if !initialized {
			last = c.LastTransitionTime
			initialized = true
		}
		if last.Before(c.LastTransitionTime) {
			last = c.LastTransitionTime
		}
		if _, ok := ret[c.LastTransitionTime]; !ok {
			ret[c.LastTransitionTime] = []Condition{}
		}
		ret[c.LastTransitionTime] = append(ret[c.LastTransitionTime], c)

	}
	return ret[last]

}

func GetTypedCondition(slice []Condition, conditionType string) (Condition, int, error) {
	for i, c := range slice {
		if c.Type == conditionType {
			return c, i, nil
		}
	}
	return Condition{}, -1, errors.New("Condition with specified type is not found")
}

func Update(slice []Condition, conditionType string, message, status *string, lastUpdateTime *time.Time) ([]Condition, error) {
	c, i, err := GetTypedCondition(slice, conditionType)
	ret := time.Now()
	if err != nil {
		return nil, err
	}
	if lastUpdateTime != nil {
		ret = *lastUpdateTime
	}
	c.LastUpdateTime = ret
	if status != nil {
		c.Status = *status
	}
	if message != nil {
		c.Message = *message
	}
	slice[i] = c
	return slice, nil
}

func Transition(slice []Condition, conditionType string, lastTransitionTime *time.Time) ([]Condition, error) {
	c, i, err := GetTypedCondition(slice, conditionType)
	ret := time.Now()
	if err != nil {
		return nil, err
	}
	if lastTransitionTime != nil {
		ret = *lastTransitionTime
	}
	c.LastTransitionTime = ret
	slice[i] = c
	return slice, nil
}
