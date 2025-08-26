package ports

import "context"

// MessageConsumer — абстракция источника сообщений (Kafka).
// Требование: at-least-once — подтверждать сообщение только после успешной обработки.
type MessageConsumer interface {
	Run(ctx context.Context) error // Run — запускает цикл потребления.
	Close() error                  // Close — освобождает ресурсы (соединения, reader и т.п.).
}
