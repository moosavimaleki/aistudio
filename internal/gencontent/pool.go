package gencontent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

type Lease struct {
	TabID, Token string
	State        map[string]any
	New          bool
}
type Pool struct {
	redis       *redis.Client
	max         int
	wait, lease time.Duration
	namespace   string
}

func NewPool(url string, max int, wait, lease time.Duration) (*Pool, error) {
	options, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return &Pool{redis: redis.NewClient(options), max: max, wait: wait, lease: lease, namespace: "gencontent:tabs"}, nil
}
func (p *Pool) Close() error           { return p.redis.Close() }
func (p *Pool) key(name string) string { return p.namespace + ":" + name }
func (p *Pool) Acquire(ctx context.Context) (Lease, error) {
	deadline := time.Now().Add(p.wait)
	for {
		lease, err := p.tryAcquire(ctx)
		if err != nil {
			return Lease{}, err
		}
		if lease.Token != "" {
			return lease, nil
		}
		if time.Now().After(deadline) {
			return Lease{}, fmt.Errorf("Tab pool is full after waiting %s", p.wait)
		}
		select {
		case <-ctx.Done():
			return Lease{}, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
func (p *Pool) tryAcquire(ctx context.Context) (Lease, error) {
	now := time.Now().UnixMilli()
	token := fmt.Sprintf("lease-%d", time.Now().UnixNano())
	script := redis.NewScript(`local expired=redis.call('ZRANGEBYSCORE',KEYS[5],'-inf',ARGV[1]);for _,id in ipairs(expired) do redis.call('HDEL',KEYS[4],id);redis.call('ZREM',KEYS[5],id);redis.call('LPUSH',KEYS[2],id) end;local id=redis.call('RPOP',KEYS[2]);local fresh=0;if not id then local total=tonumber(redis.call('GET',KEYS[1]) or '0');if total>=tonumber(ARGV[3]) then return nil end;id=ARGV[5];redis.call('INCR',KEYS[1]);fresh=1 end;redis.call('HSET',KEYS[4],id,ARGV[4]);redis.call('ZADD',KEYS[5],ARGV[2],id);local state=redis.call('HGET',KEYS[3],id) or '{}';return {id,state,fresh,ARGV[4]}`)
	result, err := script.Run(ctx, p.redis, []string{p.key("total"), p.key("available"), p.key("states"), p.key("leases"), p.key("expirations")}, now, now+p.lease.Milliseconds(), p.max, token, fmt.Sprintf("tab-%d", time.Now().UnixNano())).Result()
	if err == redis.Nil {
		return Lease{}, nil
	}
	if err != nil {
		return Lease{}, err
	}
	values := result.([]interface{})
	id := fmt.Sprint(values[0])
	state := map[string]any{}
	_ = json.Unmarshal([]byte(fmt.Sprint(values[1])), &state)
	return Lease{TabID: id, Token: fmt.Sprint(values[3]), State: state, New: fmt.Sprint(values[2]) == "1"}, nil
}
func (p *Pool) Release(ctx context.Context, lease Lease, state map[string]any) error {
	encoded, _ := json.Marshal(state)
	script := redis.NewScript(`if redis.call('HGET',KEYS[3],ARGV[1])~=ARGV[2] then return 0 end;redis.call('HDEL',KEYS[3],ARGV[1]);redis.call('ZREM',KEYS[4],ARGV[1]);redis.call('HSET',KEYS[2],ARGV[1],ARGV[3]);redis.call('LPUSH',KEYS[1],ARGV[1]);return 1`)
	result, err := script.Run(ctx, p.redis, []string{p.key("available"), p.key("states"), p.key("leases"), p.key("expirations")}, lease.TabID, lease.Token, string(encoded)).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return fmt.Errorf("Lease for tab %s expired before release", lease.TabID)
	}
	return nil
}
func (p *Pool) Discard(ctx context.Context, lease Lease) error {
	script := redis.NewScript(`if redis.call('HGET',KEYS[3],ARGV[1])~=ARGV[2] then return 0 end;redis.call('HDEL',KEYS[3],ARGV[1]);redis.call('ZREM',KEYS[4],ARGV[1]);redis.call('HDEL',KEYS[2],ARGV[1]);redis.call('DECR',KEYS[5]);return 1`)
	_, err := script.Run(ctx, p.redis, []string{p.key("available"), p.key("states"), p.key("leases"), p.key("expirations"), p.key("total")}, lease.TabID, lease.Token).Result()
	return err
}
func (p *Pool) Stats(ctx context.Context) map[string]any {
	total, _ := p.redis.Get(ctx, p.key("total")).Int()
	available, _ := p.redis.LLen(ctx, p.key("available")).Result()
	leased, _ := p.redis.HLen(ctx, p.key("leases")).Result()
	return map[string]any{"total": total, "available": available, "leased": leased, "max": p.max}
}
