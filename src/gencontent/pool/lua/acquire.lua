local expired = redis.call('ZRANGEBYSCORE', KEYS[5], '-inf', ARGV[1])
for _, tab_id in ipairs(expired) do
  redis.call('HDEL', KEYS[3], tab_id)
  redis.call('HDEL', KEYS[4], tab_id)
  redis.call('LREM', KEYS[2], 0, tab_id)
  redis.call('ZREM', KEYS[5], tab_id)
  local total = tonumber(redis.call('GET', KEYS[1]) or '0')
  if total > 0 then redis.call('DECR', KEYS[1]) end
end

while true do
  local tab_id = redis.call('LPOP', KEYS[2])
  if not tab_id then break end
  local state = redis.call('HGET', KEYS[3], tab_id)
  local leased = redis.call('HGET', KEYS[4], tab_id)
  if state and not leased then
    redis.call('HSET', KEYS[4], tab_id, ARGV[4])
    redis.call('ZADD', KEYS[5], ARGV[2], tab_id)
    return {tab_id, state, '0'}
  end
end

local total = tonumber(redis.call('GET', KEYS[1]) or '0')
if total < tonumber(ARGV[3]) then
  redis.call('INCR', KEYS[1])
  redis.call('HSET', KEYS[4], ARGV[5], ARGV[4])
  redis.call('ZADD', KEYS[5], ARGV[2], ARGV[5])
  return {ARGV[5], '', '1'}
end
return {}
