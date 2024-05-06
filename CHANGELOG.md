## Unreleased

* Set in the outbound request `X-Forwarded-For` (to the client IP address), `X-Forwarded-Host` (to the host name requested by the client), and `X-Forwarded-Proto` (to "http" or "https" depending on whether the inbound request was made on a TLS-enabled connection) (#29)

## v0.1.4 / 2024-04-26

* [BREAKING] Rename the `SSL_DOMAIN` env to `TLS_DOMAIN` (#13)
* Set `stdin` in upstream process (#18)

## v0.1.3 / 2024-03-21

* Disable transparent proxy compression (#11)

## v0.1.2 / 2024-03-19

* Don't cache `Range` requests

## v0.1.1 / 2024-03-18

* Ensure `Content-Length` set correctly in `X-Sendfile` responses (#10)

## v0.1.0 / 2024-03-07

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
