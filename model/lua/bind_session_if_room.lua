-- Atomic check-and-bind for session binding: only binds if active count < maxSessions or session is already bound.
-- KEYS[1] = channel_session_binding:{channelId}
-- ARGV[1] = sessionId (string)
-- ARGV[2] = now (string, unix timestamp)
-- ARGV[3] = maxSessions (string, integer)
-- ARGV[4] = cutoff (string, unix timestamp; 0 means no expiry)
-- ARGV[5] = userId (string)
-- Hash value format: "userId,lastUsedTime"
-- Returns: 1 if bound (or already bound), 0 if channel is full

local key = KEYS[1]
local sessionId = ARGV[1]
local now = ARGV[2]
local maxSessions = tonumber(ARGV[3])
local cutoff = tonumber(ARGV[4])
local userId = ARGV[5]

-- Check if session is already bound and active
local existing = redis.call('HGET', key, sessionId)
if existing then
    local sep = string.find(existing, ',')
    if sep then
        local lastUsed = tonumber(string.sub(existing, sep + 1))
        if cutoff == 0 or (lastUsed and lastUsed >= cutoff) then
            -- Already actively bound, just update timestamp
            redis.call('HSET', key, sessionId, userId .. ',' .. now)
            redis.call('EXPIRE', key, 86400)
            return 1
        end
    end
end

-- Count active bindings
local all = redis.call('HGETALL', key)
local activeCount = 0
for i = 1, #all, 2 do
    local val = all[i + 1]
    local sep = string.find(val, ',')
    if sep then
        local ts = tonumber(string.sub(val, sep + 1))
        if cutoff == 0 or (ts and ts >= cutoff) then
            activeCount = activeCount + 1
        end
    end
end

-- Check capacity
if activeCount >= maxSessions then
    return 0
end

-- Bind session
redis.call('HSET', key, sessionId, userId .. ',' .. now)
redis.call('EXPIRE', key, 86400)
return 1
