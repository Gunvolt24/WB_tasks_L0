//go:build integration

package testutil

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

// UniqueTopicAndGroup — даёт уникальные topic/group на основе базового префикса.
// Пример: base="orders-itest" → "orders-itest-20250826T010203123456789".
func UniqueTopicAndGroup(base string) (topic, group string) {
	// наносекунды включаем в строку и убираем точку, чтобы тема была валидной
	s := time.Now().UTC().Format("20060102T150405.000000000")
	s = strings.ReplaceAll(s, ".", "")
	return fmt.Sprintf("%s-%s", base, s), fmt.Sprintf("%s-%s", base, s)
}

// EnsureTopic — создаёт топик (если он уже есть — это OK) и ждёт его готовности.
// Параметр broker может быть:
//   - "host:port"
//   - "PLAINTEXT://host:port" (как отдаёт testcontainers)
//   - "host1:port1,host2:port2" (берётся первый)
func EnsureTopic(ctx context.Context, broker, topic string) error {
	addr := firstBootstrap(broker)

	// подключаемся к любому брокеру
	conn, err := kafka.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// находим контроллер кластера и открываем admin-коннект к нему
	ctrl, err := conn.Controller()
	if err != nil {
		return err
	}
	adminAddr := net.JoinHostPort(ctrl.Host, strconv.Itoa(ctrl.Port))

	admin, err := kafka.Dial("tcp", adminAddr)
	if err != nil {
		return err
	}
	defer admin.Close()

	// создаём топик (если уже есть — это не ошибка)
	err = admin.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
	if err != nil {
		// В разных кластерах формулировка может отличаться — проверяем подстроку.
		low := strings.ToLower(err.Error())
		if !strings.Contains(low, "already exists") {
			return err
		}
	}

	// ждём появления в метаданных
	return waitTopicReady(ctx, addr, topic)
}

// ---- helpers ----

// firstBootstrap берёт первый адрес из bootstrap-строки,
// а также снимает схему вида "PLAINTEXT://".
func firstBootstrap(raw string) string {
	// список брокеров?
	parts := strings.Split(raw, ",")
	first := strings.TrimSpace(parts[0])

	// есть схема?
	if strings.Contains(first, "://") {
		// url.Parse справится и с "PLAINTEXT://"
		if u, err := url.Parse(first); err == nil && u.Host != "" {
			return u.Host
		}
	}
	return first
}

func waitTopicReady(ctx context.Context, broker, topic string) error {
	deadline := time.Now().Add(5 * time.Second)
	for {
		// уважим контекст
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		c, err := kafka.Dial("tcp", broker)
		if err == nil {
			parts, perr := c.ReadPartitions(topic)
			_ = c.Close()
			if perr == nil && len(parts) > 0 {
				return nil
			}
			err = perr
		}

		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("topic %q not ready: %w", topic, err)
			}
			return fmt.Errorf("topic %q not ready", topic)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
