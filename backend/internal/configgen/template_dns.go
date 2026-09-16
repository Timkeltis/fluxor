package configgen

// DnsBlock 统一注入到 config.yaml 的 DNS 配置块（fake-ip 模式 + 国内外双解析）。
const DnsBlock = `
dns:
  enable: true
  listen: 0.0.0.0:1053
  prefer-h3: true
  ipv6: true
  use-hosts: true
  respect-rules: true
  default-nameserver:
    - https://223.5.5.5/dns-query
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  fake-ip-filter:
    - '*.lan'
    - '*.local'
    - '*.localhost'
    - localhost.ptlogin2.qq.com
    - '+.stun.*.*'
    - '+.stun.*.*.*'
    - '+.stun.*.*.*.*'
    - lens.l.google.com
    - '*.srv.nintendo.net'
    - +.stun.playstation.net
    - 'xbox.*.*.microsoft.com'
    - '*.*.xboxlive.com'
    - +.msftncsi.com
    - +.msftconnecttest.com
  nameserver:
    - https://120.53.53.53/dns-query
    - https://223.5.5.5/dns-query
  proxy-server-nameserver:
    - https://120.53.53.53/dns-query
    - https://223.5.5.5/dns-query
`
