# MGL03 HomeKit BLE 브리지

[English](README.en.md) · [변경 이력](CHANGELOG.md)

Xiaomi Gateway 3(`lumi.gateway.mgl03`)에서 직접 실행되는 소형 브리지로,
Home Assistant가 필요하지 않습니다. 현재 지원하는 `miaomiaoce.sensor_ht.o2` /
LYWSD02MMC 센서의 BLE 보고를 네이티브 HomeKit 온도, 습도 및 배터리
서비스로 변환합니다.

```text
miaomiaoce.sensor_ht.o2
          │ BLE
          ▼
Xiaomi Gateway 3 → openmiio_agent → 로컬 MQTT → 이 브리지 → Apple 홈
```

## 지원 기능

- Xiaomi 제품을 정확히 식별: `pdid=5860`, 모델 `miaomiaoce.sensor_ht.o2`.
- 온도, 습도, 배터리 잔량 및 배터리 부족 상태.
- 새 설치 시 30초 검색 창에서 지원 센서를 자동 등록. MAC 주소를 미리 알 필요가 없습니다.
- 재시작 후에도 센서 식별 정보와 HomeKit 페어링 유지.
- 중복·역순 BLE 프레임 제거와 MQTT/센서 오프라인 상태 표시.
- MGL03용 네이티브 Linux MIPSLE/soft-float 바이너리.
- Home Assistant, Python 런타임, Node.js 런타임 또는 외부 MQTT 라이브러리가
  필요하지 않습니다.

게이트웨이는 Apple의 홈 허브가 아니라 HomeKit **액세서리 브리지**로
동작합니다. 같은 LAN에서는 Apple 홈을 통한 로컬 제어가 가능합니다. 원격
접속과 홈 자동화에는 홈 허브로 설정된 Apple TV 또는 HomePod가 여전히
필요합니다.

## 빌드

Go 1.22 이상이 필요합니다. 58 MiB MGL03용 릴리스 바이너리는 Go 1.25.12와
`GOMIPS=softfloat` 설정으로 빌드합니다.

```powershell
go test ./...
./scripts/build.ps1 -Version 0.1.0
```

MGL03 바이너리는 `bin/mgl03-homekit-bridge`에 생성됩니다. Linux/macOS
사용자는 `make test build-mgl03`을 실행할 수 있습니다.

## 설치

[docs/INSTALL.md](docs/INSTALL.md)를 참고하십시오. 전체 과정은 다음과
같습니다.

1. MIPSLE 브리지 바이너리를 빌드합니다.
2. MGL03 펌웨어 `1.5.0`부터 `1.5.4`까지는 같은 LAN의 PC에서 Telnet 없는
   설치 프로그램을 실행하고, 숨김 프롬프트에 32자 miIO 토큰을 입력합니다.

   ```powershell
   py -m pip install -r requirements-installer.txt
   py .\scripts\install_no_telnet.py --gateway-ip 192.168.10.41
   ```

3. 센서의 다음 BLE 광고를 기다립니다.
4. 새로 설치한 경우 Apple 홈에 무작위 페어링 코드를 입력합니다.

설치 프로그램은 펌웨어의 로컬 miIO `set_ip_info` 경로를 사용하여 게이트웨이가
PC의 임시 HTTP 서버에서 SHA-256(구형 BusyBox에서는 MD5 대체)으로 검증된
자격 증명 없는 번들을 가져오게 합니다.
Telnet 세션을 열거나 TCP 23번 포트를 활성화하지 않습니다. 업데이트 중에도
기존 `/data/mgl03-homekit` 구성, 센서 및 HomeKit 페어링을 보존합니다. 지원
펌웨어 범위, 롤백 동작 및 수동 대체 방법은 설치 안내서를 참고하십시오.

첫 실행 시 안전한 무작위 페어링 PIN을 만들고 30초 동안 일치하는 센서를
검색합니다. PIN은 새 구성을 만든 최초 실행에만 권한이 제한된 로그에 기록됩니다.
편집 가능한 예제는
[configs/config.example.json](configs/config.example.json)에 있습니다.

`discovery.mode`는 `auto`, `first`, `manual` 중 하나입니다. `auto`는 시작 시
검색 창에서 여러 센서를 등록하고, 실행 중 새 센서를 발견하면 레지스트리에
저장하여 다음 재시작 때 HomeKit에 노출합니다. 이전 버전에서 생성된 구성은
호환성을 위해 `first` 방식으로 계속 동작합니다. MQTT 연결이 잠시 끊겨도
HomeKit에는 마지막으로 확인된 센서 값을 유지하고 백그라운드에서 재연결합니다.

## 설계 참고 사항

`openmiio_agent`는 기본 펌웨어 경로에 따라 MGL03 Bluetooth 서비스의 JSON을
`central/report` 또는 `miio/report`에 게시합니다. 브리지는 두 토픽을 모두
구독하고 PDID 5860 BLE 이벤트만 허용하며, 검증된 XiaomiGateway3 매핑으로
데이터를 해석합니다. 장치 DID를 확인한 뒤에는 MIoT 속성 업데이트도 허용합니다.
자세한 내용과 샘플 페이로드는 [docs/PROTOCOL.md](docs/PROTOCOL.md)에 있습니다.

HomeKit은 경량 [`github.com/brutella/hap`](https://github.com/brutella/hap)
라이브러리로 구현합니다. 첫 번째 HomeKit 액세서리는 브리지이며, 센서는 MAC
주소에서 파생한 안정적인 액세서리 ID를 유지합니다. 번들에 포함된 dnssd
v1.2.14 패치는 Linux에서 주소 재사용을 활성화하므로, 브리지가 MGL03 기본
HomeKit 서비스와 mDNS 5353번 포트를 공유할 수 있습니다. 기본 서비스를 중지할
필요가 없습니다.

## 보안 및 복구

- openmiio의 인증 없는 MQTT 1883번 포트는 LAN에서 접근 가능하므로 신뢰할 수
  있는 IoT VLAN에 두고 라우터에서 포워딩하지 마십시오. 브리지 자체는
  `127.0.0.1`로만 접속합니다.
- 펌웨어 또는 브리지를 업그레이드하기 전에 `/data/mgl03-homekit`을
  백업하십시오.
- 펌웨어 업데이트로 사용자 지정 시작 훅이 제거되거나 로컬 miIO 설치 경로가
  비활성화될 수 있습니다.
- Telnet 없는 설치 프로그램은 의도적으로 MGL03 펌웨어 `1.5.0`-`1.5.4`만
  지원하며, 다른 모델과 버전에서는 실행을 거부합니다.
- 이 프로젝트는 읽기 전용 펌웨어를 패치하지 않습니다.

## 업스트림 참고 자료

- [openmiio_agent](https://github.com/AlexxIT/openmiio_agent)
- [XiaomiGateway3](https://github.com/AlexxIT/XiaomiGateway3)
- [brutella/hap](https://github.com/brutella/hap)
