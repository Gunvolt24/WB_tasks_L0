//go:generate mockgen -source=../order_repository.go -destination=./mock_order_repository.go -package=mocks
//go:generate mockgen -source=../order_cache.go      -destination=./mock_order_cache.go      -package=mocks
//go:generate mockgen -source=../validator.go        -destination=./mock_validator.go        -package=mocks
//go:generate mockgen -source=../logger.go           -destination=./mock_logger.go           -package=mocks
//go:generate mockgen -source=../message_consumer.go -destination=./mock_message_consumer.go -package=mocks
//go:generate mockgen -source=../order_read_service.go -destination=mock_order_read_service.go -package=mocks

package mocks
