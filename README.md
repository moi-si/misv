# misv

A simple proxying static file server.

## Overview

misv is a lightweight HTTP server for serving static files from a local directory, with the ability to automatically fetch missing files from an origin server via HTTPS. It supports optional SOCKS5 proxying and custom HTTP headers.

## Features

- Serves **static** files from a specified local directory.
- Missing files are fetched cached from a remote HTTPS origin automatically.
- SOCKS5 proxy support for outbound requests.
- Custom User-Agent and X-Forwarded-For headers.
- Automatic creation of missing directories.
- Simple CLI configuration.

## Usage

Build the project with Go:

```bash
go build -o misv
```

Run the server:

```bash
./misv -bind 127.0.0.1:1225 -origin example.com [options]
```

### Arguments

| Argument  | Required | Description                                               |
| --------- | -------- | --------------------------------------------------------- |
| `-bind`   | Yes      | Address to listen on, e.g., `0.0.0.0:1997`                |
| `-origin` | Yes      | Origin server domain, e.g., `example.com`                 |
| `-root`   | No       | Local root directory for files (defaults to `./<origin>`) |
| `-socks5` | No       | SOCKS5 proxy address, e.g., `127.0.0.1:1080`              |
| `-ua`     | No       | Custom User-Agent header for origin requests              |
| `-xff`    | No       | Custom X-Forwarded-For header for origin requests         |

### Example

Start the server, listening on port 1225, proxying and caching files for example.com:

```bash
./misv -bind :1225 -origin example.com
```

Use a SOCKS5 proxy:

```bash
./misv -bind 127.0.0.1:1080 -origin example.com -socks5 127.0.0.1:1080
```

Set custom headers:

```bash
./misv -bind 127.0.0.1:1997 -origin example.com -ua CustomAgent/1.0 -xff 20.24.1.17
```

Specify a custom root directory:

```bash
./misv -bind :1997 -origin example.com -root /data/misv-cache
```

## How It Works

1. Receives HTTP **GET** requests.
2. Tries to serve the corresponding file from the local root directory.
3. If the file is not found, fetches it using **HTTPS** from the specified origin server, then caches it locally.
4. Optionally uses SOCKS5 proxy and custom headers for the fetch request.
5. Serves the requested file to the client.

## Notes

- Only HTTP GET requests are supported.
- If the requested path is a directory, it attempts to serve `index.html`.
- The local cache will grow over time as files are requested and downloaded from the origin.
- Errors during fetch or save are returned as HTTP 500/502 responses.

## License

GPLv3
