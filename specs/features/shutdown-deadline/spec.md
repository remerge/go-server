# Shutdown Deadline

Stop closes listeners and sockets, then waits for handlers.
The wait ends when handlers finish or StopDeadline expires.
Expiry logs the existing warning and returns without cancelling handlers.
Timeout values and earlier listener/socket teardown are unchanged.
Real TCP tests cover completion, idle connections and stuck handlers.
