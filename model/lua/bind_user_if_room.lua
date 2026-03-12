-- Atomic check-and-bind: only binds if active count < maxUsers or user is already bound.
-- KEYS[1] = channel_binding:{channelId}
-- ARGV[1] = userId (string)
-- ARGV[2] = now (string, unix timestamp)
-- ARGV[3] = maxUsers (string, integer)
-- ARGV[4] = cutoff (string, unix timestamp; 0 means no expiry)
-- Returns: 1 if bound (or already bound), 0 if channel is full

local key = KEYS[1]
local userId = ARGV[1]
local now = ARGV[2]
local maxUsers = tonumber(ARGV[3])
local cutoff = tonumber(ARGV[4])

-- Check if user is already bound and active
local existing = redis.call('HGET', key, userId)
if existing then
    local lastUsed = tonumber(existing)
    if cutoff == 0 or lastUsed >= cutoff then
        -- Already actively bound, just update timestamp
        redis.call('HSET', key, userId, now)
        redis.call('EXPIRE', key, 86400)
        return 1
    end
end

-- Count active bindings (excluding the current user's expired entry if any)
local all = redis.call('HGETALL', key)
local activeCount = 0
for i = 1, #all, 2 do
    local ts = tonumber(all[i + 1])
    if cutoff == 0 or ts >= cutoff then
        activeCount = activeCount + 1
    end
end

-- Check capacity
if activeCount >= maxUsers then
    return 0
end

-- Bind user
redis.call('HSET', key, userId, now)
redis.call('EXPIRE', key, 86400)
return 1
