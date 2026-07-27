package main

type publisher interface{ Publish(snapshot) error }

type service struct {
	store     *alertStore
	policy    filterPolicy
	publisher publisher
}

type processingResult struct {
	transition
	Ignored       int
	IgnoreReasons map[string]int
	Published     bool
}

func (s *service) process(alerts []alert) (processingResult, error) {
	accepted := make([]normalizedAlert, 0, len(alerts))
	result := processingResult{IgnoreReasons: make(map[string]int)}
	for _, input := range alerts {
		alert, reason, ok := s.policy.normalize(input)
		if !ok {
			result.Ignored++
			result.IgnoreReasons[reason]++
			continue
		}
		accepted = append(accepted, alert)
	}
	result.transition = s.store.apply(accepted)
	current, needed := s.store.needsPublish()
	if !needed {
		return result, nil
	}
	if err := s.publisher.Publish(current); err != nil {
		return result, err
	}
	s.store.markPublished(current)
	result.Published = true
	return result, nil
}
