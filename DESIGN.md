# Next steps

- Health checks to backends - separate background goroutine?
- Token Bucket Rate Limiting for clients (TCP version)
- Use atomic instead of mutex for `nextBackend` counter to reduce overhead at scale
- io.Copy ownership is split between `handleConn` and `sync.Once` in proxy function now - revert it back to `go io.Copy` and `io.Copy` & add `defer clientConn.Close()` to handleConn function
- Replace `io.Copy` with read write buf with idle timeout
- Connection pool? (but with chat applications?)