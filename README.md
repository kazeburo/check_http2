# check_http2

Nagios check_http plugin alternative.

Currentry not implment full feature, only we need.

## Usage

```
./check_http2 -h
Usage:
  check_http2 [OPTIONS]

Application Options:
      --timeout=       Timeout to wait for connection (default: 300s)
  -H, --hostname=      Host name using Host headers
  -I, --IP-address=    IP address or name
  -p, --port=          Port number
  -j, --method=        Set HTTP Method (default: GET)
  -u, --uri=           URI to request (default: /)
  -e, --expect=        Comma-delimited list of expected HTTP response status (default: HTTP/1.,HTTP/2.)
  -A, --useragent=     UserAgent to be sent (default: check_http)
  -a, --authorization= username:password on sites with basic authentication
  -S, --ssl            use https
      --sni            enable SNI
  -4                   use tcp4 only
  -6                   use tcp6 only
  -v, --version        Show version

Help Options:
  -h, --help           Show this help message
```

example

```
% ./check_http2 -S  -I blog.nomadscafe.jp -H blog.nomadscafe.jp -u /2016/03/retty-tech-cafe-5.html -e 'HTTP/1.0 200,HTTP/1.1 200,HTTP/2.0 200' -j HEAD --sni
HTTP OK: Status line output matched "HTTP/2.0 200"  - 482 bytes in 0.183 second response time | time=0.182730s;;;0.000000 size=482B;;;0
```

