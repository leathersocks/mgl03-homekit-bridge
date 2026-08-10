# Bonjour 적합성 테스트

[English](README.en.md)

목표는 `dnssd`가 Apple의 [Bonjour](https://developer.apple.com/bonjour/)를
완전히 준수하게 하는 것입니다. mDNS 응답기 구현을 테스트하기 위해 *Bonjour
Conformance Test*(v1.5.0)를 사용합니다.

테스트 컴퓨터로 라우터에 연결된 macOS 10.12 이상 MacBook Pro를 사용합니다.
테스트 대상 장치는 같은 라우터에 연결된 Raspberry Pi 3 Model B입니다.

`_cmd/bct/main.go`에는 mDNS 응답기 테스트 구현이 있으며,
`GOOS=linux GOARCH=arm GOARM=7 go build -o bct main.go` 명령으로 RPi용으로
컴파일합니다. RPi에서 `bct` 실행 파일을 실행하는 동시에 테스트 컴퓨터에서
`sudo ./BonjourConformanceTest -S -M h -DD -E <router-ip>` 명령으로 멀티캐스트
DNS 테스트를 실행합니다(핫 플러깅 제외, #9 참고).

최신 테스트 결과는 `ConformanceTestResults`에서 확인할 수 있습니다.
