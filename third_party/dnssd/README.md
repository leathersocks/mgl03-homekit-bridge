# DNS-SD

[English](README.en.md)

[![빌드 상태](https://travis-ci.org/brutella/hc.svg)](https://travis-ci.org/brutella/dnssd)

이 라이브러리는 [멀티캐스트 DNS][mdns]와 [DNS 기반 서비스 검색][dnssd]을
구현하여 별도 설정 없는 동작을 제공합니다. 특정 링크 로컬 도메인에서 서비스를
알리고 찾을 수 있습니다.

[mdns]: https://tools.ietf.org/html/rfc6762
[dnssd]: https://tools.ietf.org/html/rfc6763

## 사용법

#### mDNS 응답기 만들기

다음 코드는 `eth0` 네트워크 인터페이스의 모든 IP를 사용하는 호스트
"My Computer"에 "My Website._http._tcp.local."이라는 서비스를 만들고
응답기에 추가합니다.

```go
import (
	"context"
	"github.com/brutella/dnssd"
)

cfg := dnssd.Config{
    Name:   "My Website",
    Type:   "_http._tcp",
    Domain: "local",
    Host:   "My Computer",
    Ifaces: []string{"eth0"},,
    Port:   12345,
}
sv, _ := dnssd.NewService(cfg)
```

대부분의 경우 서비스의 이름, 유형 및 포트만 지정하면 됩니다.

```go
cfg := dnssd.Config{
    Name:   "My Website",
    Type:   "_http._tcp",
    Port:   12345,
}
sv, _ := dnssd.NewService(cfg)
```

그런 다음 응답기를 만들고 서비스를 추가합니다.

```go
rp, _ := dnssd.NewResponder()
hdl, _ := rp.Add(sv)

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

rp.Respond(ctx)
```

`Respond`를 호출하면 응답기는 서비스 인스턴스 이름과 호스트 이름이 네트워크에서
고유한지 조사합니다. 조사가 끝나면 서비스를 알립니다.

#### TXT 레코드 업데이트

서비스를 응답기에 추가한 뒤 `hdl`을 사용하여 속성을 업데이트할 수 있습니다.

```go
hdl.UpdateText(map[string]string{"key1": "value1", "key2": "value2"}, rsp)
```

## `dnssd` 명령

`cmd/dnssd`의 명령줄 도구를 사용하면
[dns-sd](https://www.unix.com/man-page/osx/1/dns-sd/)와 비슷하게 서비스를
탐색하고 등록하고 확인할 수 있습니다.

### 설치

다음 명령으로 도구를 설치할 수 있습니다.

`go install github.com/brutella/dnssd/cmd/dnssd`

### 사용법

**로컬 컴퓨터에 서비스 등록**

로컬 컴퓨터의 515번 포트에서 실행되는 프린터 서비스(`_printer._tcp`)를
"Private Printer"라는 이름으로 등록합니다.

```sh
dnssd register -Name="Private Printer" -Type="_printer._tcp" -Port=515
```

**프록시 서비스 등록**

서비스가 로컬 네트워크의 다른 컴퓨터에서 실행 중이면 호스트 이름과 IP를
지정해야 합니다. 프린터 서비스가 호스트 이름 `ABCD`, IPv4 주소
`192.168.1.53`인 프린터에서 실행 중이라고 가정하면, 네트워크에 해당 서비스를
알리는 프록시를 등록할 수 있습니다.

```sh
dnssd register -Name="Private Printer" -Type="_printer._tcp" -Port=515 -IP=192.168.1.53 -Host=ABCD
```

특정 네트워크 인터페이스에서만 서비스를 알리려면 `-Interface` 옵션을
사용하십시오. 로컬 컴퓨터가 여러 서브넷에 연결되어 있고 알린 서비스를 특정
서브넷에서만 사용할 수 있을 때 필요할 수 있습니다.

```sh
dnssd register -Name="Private Printer" -Type="_printer._tcp" -Port=515 -IP=192.168.1.53 -Host=ABCD -Interface=en0
```

**서비스 탐색**

서비스 유형을 탐색하려면 `browse` 명령을 사용합니다.

```sh
dnssd browse -Type="_printer._tcp"
```

**서비스 인스턴스 확인**

서비스 인스턴스의 이름을 알고 있다면 `resolve` 명령으로 호스트 이름을 확인할
수 있습니다.

```sh
dnssd resolve -Name="Private Printer" -Type="_printer._tcp"
```

## 준수 여부

이 라이브러리는 Apple Bonjour Conformance Test의
[멀티캐스트 DNS 테스트](https://github.com/brutella/dnssd/blob/36a2d8c541aab14895fc5492d5ad8ec447a67c47/_cmd/bct/ConformanceTestResults)를
통과합니다.

## 할 일

- [ ] 핫 플러깅 지원
- [ ] 부정 응답 지원(RFC6762 6.1)
- [ ] TXT 레코드를 대소문자 구분 없이 처리
- [ ] 오래된 서비스를 캐시에서 정기적으로 제거
- [ ] 호스트 이름이 FQDN인지 확인

# 연락처

Matthias Hochgatterer

Github: [https://github.com/brutella](https://github.com/brutella/)

Twitter: [https://twitter.com/brutella](https://twitter.com/brutella/)

# 라이선스

*dnssd*는 MIT 라이선스로 제공됩니다. 자세한 내용은 LICENSE 파일을
참고하십시오.
