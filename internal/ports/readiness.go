package ports

import "context"

// ReadinessChecker checks whether a required external dependency is reachable.
type ReadinessChecker interface {
	Ping(context.Context) error
}
