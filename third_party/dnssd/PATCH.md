# 로컬 dnssd 패치

[English](PATCH.en.md)

이 디렉터리에는 MIT 라이선스로 배포되는 `github.com/brutella/dnssd`
v1.2.14가 포함되어 있습니다. Linux mDNS 리스너는 바인딩 전에
`SO_REUSEADDR`을 설정하고, 가능한 경우 `SO_REUSEPORT`도 설정하도록
패치되었습니다.

Xiaomi MGL03 기본 HomeKit 서비스는 이미 UDP 5353번 포트를 수신합니다. 소켓
재사용을 통해 기본 서비스를 중지하지 않고 Bluetooth HomeKit 브리지가 해당
서비스와 함께 실행될 수 있습니다.
