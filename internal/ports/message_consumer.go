package ports

import "context"

type MessageConsumer interface {
	Run(ctx context.Context) error
	Close() error
}
