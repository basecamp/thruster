## v0.0.3 / 2024-03-06

* Build with Go 1.22.1
* Use stdlib `MaxBytesHandler` for request size limiting

## v0.0.3 / 2024-03-06

* Support additional ACME providers
* Respond with `413`, not `400` when blocking oversized requests
* Allow prefixing env vars with `THRUSTER_` to avoid naming clashes
* Additional debug-level logging

## v0.0.2 / 2024-02-28

* Support `Vary` header in HTTP caching
* Return `X-Cache` `bypass` when not caching request

## v0.0.1 / 2024-02-14

* Initial version
