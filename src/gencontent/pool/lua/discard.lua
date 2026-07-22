if redis.call('HGET', KEYS[3], ARGV[1]) ~= ARGV[2] then return 0 end
redis.call('LREM', KEYS[1], 0, ARGV[1])
redis.call('HDEL', KEYS[2], ARGV[1])
redis.call('HDEL', KEYS[3], ARGV[1])
redis.call('ZREM', KEYS[4], ARGV[1])
local total = tonumber(redis.call('GET', KEYS[5]) or '0')
if total > 0 then redis.call('DECR', KEYS[5]) end
return 1
