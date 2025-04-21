## v0.1.13 / 2025-04-21

* Update deps to address CVEs (#74)
* Build with Go 1.24.2

## v0.1.12 / 2025-03-10

* Build with Go 1.23.7 (#69)

## v0.1.11 / 2025-02-11

* Build with Go 1.23.6 (#67)
* Allow disabling compression with env var (#56)

## v0.1.10 / 2025-01-06

* Avoid runtime glibc dependency in dist builds

## v0.1.9 / 2024-11-13

* Build with Go 1.23.3

## v0.1.8 / 2024-08-06

* Only forward X-Forwarded-* by default when not using TLS

## v0.1.7 / 2024-07-11

* Preserve existing X-Forwarded-* headers when present

## v0.1.6 / 2024-07-10

* Properly handle an empty TLS_DOMAIN value

## v0.1.5 / 2024-07-09

* Fix bug where replacing existing cache items could lead to a crash during
  eviction
* Accept comma-separated `TLS_DOMAIN` to support multiple domains (#28)
* Populate `X-Forwarded-For`, `X-Forwarded-Host` and `X-Forwarded-Proto`
  headers (#29)

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
