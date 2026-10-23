# Shutdown Deadline

After listener shutdown and socket closure, Stop waits for handlers until they finish or StopDeadline expires.
Deadline expiry logs the existing warning and returns instead of continuing to wait indefinitely.
It does not cancel handlers, change timeout values, or bound earlier listener/socket teardown.
Real TCP tests cover ordinary completion, an idle connection, and a handler held past the deadline.
