package consumer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
)

type Repo struct {
	redisClient *redis.Client
	quorumSize  uint
}

func NewRepo(redisClient *redis.Client, quorumSize uint) *Repo {
	return &Repo{
		redisClient: redisClient,
		quorumSize:  quorumSize,
	}
}

const TTLMins12 = 12 * time.Minute
const coolDownTemplate = "cooldown:%s"

func (r *Repo) SetSendingStatus(ctx context.Context, countKey, statusKey string) (bool, error) {
	luaScript := `
        local count = tonumber(redis.call("GET", KEYS[1]))
        if count and count >= tonumber(ARGV[1]) then
            local status = redis.call("GET", KEYS[2])
            if not status or status == ARGV[2] then
                redis.call("SET", KEYS[2], ARGV[3])
				redis.call("EXPIRE", KEYS[2], ARGV[4])
                return 1
            end
        end
        return 0
    `

	keys := []string{countKey, statusKey}
	args := []any{
		r.quorumSize,
		string(StatusNotSend),
		string(StatusSending),
		TTLMins12,
	}

	res, err := r.redisClient.Eval(ctx, luaScript, keys, args).Result()
	if err != nil {
		return false, fmt.Errorf(`could not get notification_sent_status: %v`, err)
	}

	sending, ok := res.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected notification_sent_status: %v", res)
	}

	return sending == 1, nil
}

func (r *Repo) SeStatusSent(ctx context.Context, statusKey string) error {
	return r.redisClient.Set(ctx, statusKey, string(StatusSent), TTLMins10).Err()
}

func (r *Repo) GetStatus(ctx context.Context, statusKey string) (Status, error) {
	status, err := r.redisClient.Get(ctx, statusKey).Result()
	if errors.Is(err, redis.Nil) {
		return StatusNotSend, nil
	} else if err != nil {
		return "", fmt.Errorf("could not get status for key %s: %w", statusKey, err)
	}

	return Status(status), nil
}

func (r *Repo) SetCoolDown(ctx context.Context, key string) error {
	return r.redisClient.Set(ctx, fmt.Sprintf(coolDownTemplate, key), "", TTLMins30).Err()
}

func (r *Repo) GetCoolDown(ctx context.Context, key string) (bool, error) {
	exists, err := r.redisClient.Exists(ctx, fmt.Sprintf(coolDownTemplate, key)).Result()
	if err != nil {
		return false, err
	}

	if exists == 1 {
		return true, nil
	}

	return false, nil
}

func (r *Repo) AddIntoStream(ctx context.Context, finding []byte, sender notifiler.FindingSender) (string, error) {
	message := map[string]interface{}{
		"channelType": string(sender.GetType()),
		"channelId":   sender.GetChannelID(),
		"finding":     string(finding),
	}

	id, err := r.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: sender.GetRedisStreamName(),
		Values: message,
	}).Result()

	if err != nil {
		return "", fmt.Errorf("failed to add message to stream: %v", err)
	}

	return id, nil
}
