# Thruster

Thruster is an HTTP/2 proxy for simple production-ready deployments of Rails
applications. It runs alongside the Puma webserver to provide a few additional
features to help your app run efficiently and safely on the open Internet:

- Automatic SSL certificate management with Let's Encrypt
- HTTP/2 support
- Basic HTTP caching
- X-Sendfile support for efficient file serving
- Automatic GZIP compression

Thruster tries to be as zero-config as possible, so most features are
automatically enabled with sensible defaults.

One exception to that is the `SSL_DOMAIN` environment variable, which is
required to enable SSL provisioning. If `SSL_DOMAIN` is not set, Thruster will
operate in HTTP-only mode.

## Installation

Add this line to your application's Gemfile:

```ruby
gem 'thruster'
```

Or install it globally:

```sh
$ gem install thruster
```

## Usage

To run your Puma application inside Thruster, prefix your usual command string
with `thrust`. For example:

```sh
$ thrust bin/rails server
```

Or with automatic SSL:

```sh
$ SSL_DOMAIN=myapp.example.com thrust bin/rails server
```

## Custom configuration

Thruster provides a number of environment variables that can be used to
customize its behavior:

- `SSL_DOMAIN` - The domain name to use for SSL provisioning. If not set, SSL
  will be disabled.

- `TARGET_PORT` - The port that your Puma server should run on. Defaults to
  3000. Thruster will set `PORT` to this when starting your server.

- `CACHE_SIZE` - The size of the HTTP cache in bytes. Defaults to 64MB.

- `MAX_CACHE_ITEM_SIZE` - The maximum size of a single item in the HTTP cache
  in bytes. Defaults to 1MB.

- `X_SENDFILE_ENABLED` - Whether to enable X-Sendfile support. Defaults to
  enabled; set to `0` or `false` to disable.

- `MAX_REQUEST_BODY` - The maximum size of a request body in bytes. Requests
  larger than this size will be refused; `0` means no maximum size. Defaults to
  `0`.

- `STORAGE_PATH` - The path to store Thruster's internal state. Defaults to
  `./storage/thruster`.

- `BAD_GATEWAY_PAGE` - Path to an HTML file to serve when the backend server
  returns a 502 Bad Gateway error. Defaults to `./public/502.html`. If there is
  no file at the specific path, Thruster will serve an empty 502 response
  instead.

- `HTTP_PORT` - The port to listen on for HTTP traffic. Defaults to 80.

- `HTTPS_PORT` - The port to listen on for HTTPS traffic. Defaults to 443.

- `HTTP_IDLE_TIMEOUT` - The maximum time in seconds that a client can be idle
  before the connection is closed. Defaults to 60.

- `HTTP_READ_TIMEOUT` - The maximum time in seconds that a client can take to
  send the request headers. Defaults to 30.

- `HTTP_WRITE_TIMEOUT` - The maximum time in seconds during which the client
must read the response. Defaults to 30.
