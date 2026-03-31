# Next steps

- Health checks to backends - separate background goroutine? (DONE - recheck)
- Token Bucket Rate Limiting for clients (TCP version) (DONE)
- Use atomic instead of mutex for `nextBackend` counter to reduce overhead at scale (DONE)
- io.Copy ownership is split between `handleConn` and `sync.Once` in proxy function now - revert it back to `go io.Copy` and `io.Copy` & add `defer clientConn.Close()` to handleConn function (DONE)
- Replace `io.Copy` with read write buf with idle timeout

- Connection pool? (but with chat applications?) or buffer
- Add idle timeout or read/write deadlines for existing connections to avoid pile up
- Backpressure (Connection limits)