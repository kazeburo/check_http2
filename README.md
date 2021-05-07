# check_http2

Nagios check_http plugin alternative.
Not implemented full feature, only we need.

## Usage

```
./check_http2 -h
Usage:
  check_http2 [OPTIONS]

Application Options:
      --timeout=              Timeout to wait for connection (default: 10s)
      --max-buffer-size=      Max buffer size to read response body (default: 1MB)
      --no-discard            raise error when the response body is larger then max-buffer-size
      --wait-for              retry until successful when enabled
      --wait-for-consecutive= number of consecutive successful requests required (default: 1)
      --wait-for-interim=     interval time after successful request (default: 1s)
      --wait-for-interval=    retry interval (default: 2s)
      --wait-for-max=         time to wait for success
  -H, --hostname=             Host name using Host headers
  -I, --IP-address=           IP address or Host name
  -p, --port=                 Port number
  -j, --method=               Set HTTP Method (default: GET)
  -u, --uri=                  URI to request (default: /)
  -e, --expect=               Comma-delimited list of expected HTTP response status (default: HTTP/1.,HTTP/2.)
  -s, --string=               String to expect in the content
      --base64-string=        Base64 Encoded string to expect the content
  -A, --useragent=            UserAgent to be sent (default: check_http)
  -a, --authorization=        username:password on sites with basic authentication
  -S, --ssl                   use https
      --sni                   enable SNI
  -4                          use tcp4 only
  -6                          use tcp6 only
  -v, --version               Show version
      --verbose               log more

Help Options:
  -h, --help                  Show this help message


```

example

check with HEAD request

```
% ./check_http2 -S  -I blog.nomadscafe.jp -H blog.nomadscafe.jp -u /2016/03/retty-tech-cafe-5.html -e 'HTTP/1.0 200,HTTP/1.1 200,HTTP/2.0 200' -j HEAD --sni
HTTP OK: Status line output "HTTP/2.0 200 OK" matched "HTTP/2.0 200"  - 482 bytes in 0.349 second response time | time=0.349428s;;;0.000000 size=482B;;;0
```

wait for success

```
% ./check_http2 -S -H blog.nomadscafe.jp -s kazeburo-wait-for --wait-for --wait-for-max 10s
2021/03/24 15:44:20 HTTP CRITICAL - HTTP response body Not matched "kazeburo-wait-for" from host on port 443
2021/03/24 15:44:22 HTTP CRITICAL - HTTP response body Not matched "kazeburo-wait-for" from host on port 443
2021/03/24 15:44:24 HTTP CRITICAL - HTTP response body Not matched "kazeburo-wait-for" from host on port 443
2021/03/24 15:44:27 HTTP CRITICAL - HTTP response body Not matched "kazeburo-wait-for" from host on port 443
2021/03/24 15:44:29 HTTP CRITICAL - HTTP response body Not matched "kazeburo-wait-for" from host on port 443
Give up waiting for success
```

wait for consecutive successful


```
% ./check_http2 --verbose -H blog.nomadscafe.jp -S -s kazeburo -p 443 --timeout 1s --wait-for --wait-for-max 10s --wait-for-consecutive 5 --wait-for-interim 100ms
2021/05/08 00:37:32 GET https://blog.nomadscafe.jp/ to blog.nomadscafe.jp:443
2021/05/08 00:37:32 HTTP OK: Status line output "HTTP/2.0 200 OK" matched "HTTP/1.,HTTP/2.", Response body matched "kazeburo"  - 106088 bytes in 0.333 second response time | time=0.332926s;;;0.000000 size=106088B;;;0
2021/05/08 00:37:32 GET https://blog.nomadscafe.jp/ to blog.nomadscafe.jp:443
2021/05/08 00:37:32 HTTP OK: Status line output "HTTP/2.0 200 OK" matched "HTTP/1.,HTTP/2.", Response body matched "kazeburo"  - 106088 bytes in 0.110 second response time | time=0.110140s;;;0.000000 size=106088B;;;0
2021/05/08 00:37:32 GET https://blog.nomadscafe.jp/ to blog.nomadscafe.jp:443
2021/05/08 00:37:32 HTTP OK: Status line output "HTTP/2.0 200 OK" matched "HTTP/1.,HTTP/2.", Response body matched "kazeburo"  - 106088 bytes in 0.113 second response time | time=0.113044s;;;0.000000 size=106088B;;;0
2021/05/08 00:37:33 GET https://blog.nomadscafe.jp/ to blog.nomadscafe.jp:443
2021/05/08 00:37:33 HTTP OK: Status line output "HTTP/2.0 200 OK" matched "HTTP/1.,HTTP/2.", Response body matched "kazeburo"  - 106088 bytes in 0.102 second response time | time=0.102190s;;;0.000000 size=106088B;;;0
2021/05/08 00:37:33 GET https://blog.nomadscafe.jp/ to blog.nomadscafe.jp:443
HTTP OK: Status line output "HTTP/2.0 200 OK" matched "HTTP/1.,HTTP/2.", Response body matched "kazeburo"  - 106088 bytes in 0.097 second response time | time=0.096845s;;;0.000000 size=106088B;;;0
```
