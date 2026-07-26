if redis.call('HGET', KEYS[3], ARGV[1]) ~= ARGV[2] then return 0 end
redis.call('HSET', KEYS[2], ARGV[1], ARGV[3])
redis.call('HDEL', KEYS[3], ARGV[1])
redis.call('ZREM', KEYS[4], ARGV[1])
redis.call('LREM', KEYS[1], 0, ARGV[1])
redis.call('RPUSH', KEYS[1], ARGV[1])
return 1
