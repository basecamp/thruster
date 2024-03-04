# Thruster

Thruster is an HTTP/2 proxy for simple production-ready deployments of Rails
applications. It runs alongside the Puma webserver to provide a few additional
features to help your app run efficiently and safely on the open Internet:

- Automatic SSL certificate management with Let's Encrypt
- HTTP/2 support
- Basic HTTP caching
- X-Sendfile support for efficient file serving
- Automatic GZIP compression
- Image proxy links to sanitize external image URLs

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

## Image proxy links

Applications that allow user-generated content often need a way to sanitize
external image URLs, to guard against the security risks of maliciously crafted
images.

Thruster includes a minimal image proxy that inspects the content of external
images before serving them. Images will be served if they:

- Appear to be valid image files
- Are in a permitted format: GIF, JPEG, PNG or WebP
- Do not have an excessive width or height (5000 pixels max, by default)

External images that do not meet these criteria will be served with a `403
Forbidden` status.

To use the image proxy, your application should rewrite external image URLs in
user-generated content to use Thruster's image proxy path. This path is provided
to your application in the `IMAGE_PROXY_PATH` environment variable. Specify the
URL of the image to proxy as a query parameter named `src`.

Thruster provides a helper method to form these paths for you:

```ruby
Thruster.image_proxy_path('https://example.com/image.jpg')
```

When your application is running outside of Thruster,
`Thruster.image_proxy_path` will return the original URL unchanged.

## Custom configuration

Thruster provides a number of environment variables that can be used to
customize its behavior.

To prevent naming clashes with your application's own environment variables,
Thruster's environment variables can optionally be prefixed with `THRUSTER_`.
For example, `SSL_DOMAIN` can also be set as `THRUSTER_SSL_DOMAIN`. Whenever a
prefixed variable is set, Thruster will use it in preference to the unprefixed
version.

| Variable Name               | Description                                                                     | Default Value |
|-----------------------------|---------------------------------------------------------------------------------|---------------|
| `SSL_DOMAIN`                | The domain name to use for SSL provisioning. If not set, SSL will be disabled.  | None |
| `TARGET_PORT`               | The port that your Puma server should run on. Thruster will set `PORT` to this when starting your server. | 3000 |
| `CACHE_SIZE`                | The size of the HTTP cache in bytes.                                            | 64MB |
| `MAX_CACHE_ITEM_SIZE`       | The maximum size of a single item in the HTTP cache in bytes.                   | 1MB |
| `X_SENDFILE_ENABLED`        | Whether to enable X-Sendfile support. Set to `0` or `false` to disable.         | Enabled |
| `IMAGE_PROXY_ENABLED`       | Whether to enable the built in image proxy. Set to `0` or `false` to disable.   | Enabled |
| `IMAGE_PROXY_MAX_DIMENSION` | When using the image proxy, only serve images with a width and height less than this, in pixels | 5000 |
| `MAX_REQUEST_BODY`          | The maximum size of a request body in bytes. Requests larger than this size will be refused; `0` means no maximum size. | `0` |
| `STORAGE_PATH`              | The path to store Thruster's internal state.                                    | `./storage/thruster` |
| `BAD_GATEWAY_PAGE`          | Path to an HTML file to serve when the backend server returns a 502 Bad Gateway error. If there is no file at the specific path, Thruster will serve an empty 502 response instead. | `./public/502.html` |
| `HTTP_PORT`                 | The port to listen on for HTTP traffic.                                         | 80 |
| `HTTPS_PORT`                | The port to listen on for HTTPS traffic.                                        | 443 |
| `HTTP_IDLE_TIMEOUT`         | The maximum time in seconds that a client can be idle before the connection is closed. | 60 |
| `HTTP_READ_TIMEOUT`         | The maximum time in seconds that a client can take to send the request headers. | 30 |
| `HTTP_WRITE_TIMEOUT`        | The maximum time in seconds during which the client must read the response.     | 30 |
| `DEBUG`                     | Set to `1` or `true` to enable debug logging.                                   | Disabled |
