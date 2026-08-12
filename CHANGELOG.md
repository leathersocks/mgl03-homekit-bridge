# 변경 이력

[English](CHANGELOG.en.md)

이 프로젝트의 주요 변경 사항을 기록합니다. 아직 버전 태그가 발행되지 않았으므로
첫 공식 릴리스 전 변경은 `Unreleased`에 정리합니다.

형식은 [Keep a Changelog](https://keepachangelog.com/ko/1.1.0/)를 따르며,
공식 릴리스부터 [유의적 버전](https://semver.org/lang/ko/)을 사용합니다.

## [Unreleased]

### 추가

- `k0918.toothbrush.t700i` / PDID 6032 Xiaomi Toothbrush T700i를 HomeKit 동작
  센서와 배터리 서비스로 제공하는 지원 기능.
- 60초 실시간 이벤트 판정, 오래된 강제 종료 패킷의 게이트웨이 timestamp 보정,
  반복 시작 heartbeat 및 30초 종료 유실 watchdog을 포함하는 T700i 양치 세션 복구.
- 다른 BLE 제품군이 MQTT 런타임을 변경하지 않고 디코더와 HomeKit 서비스 구성을
  추가할 수 있는 제품 독립적인 `DeviceAccessory`와 액세서리 팩토리 구조.
- 제품 등록, 디코더, 액세서리, 이관 및 검증 요구사항을 설명하는 한국어·영어
  장치 확장 안내서.
- SmartThings Edge처럼 HomeKit을 사용하지 않는 소비자를 위한 무텔넷
  `--mode openmiio` 설치 모드.
- openmiio/MQTT 시작과 TCP 1883 준비 확인을 담당하는 공용
  `/data/mgl03-openmiio-start.sh` 런타임 스크립트.
- `miio/report`와 `central/report`만 안전하게 구독하는 독립형
  `scripts/mqtt_ble_probe.py` 진단 도구.
- 신규 설치 시 30초 검색 창에서 지원되는 여러 BLE 센서를 자동 등록하는
  `discovery.mode=auto` 정책.
- 기존 단일 센서 동작을 유지하는 `first` 정책과 자동 등록을 끄는 `manual` 정책.
- 장치별 제품 ID, 모델 및 안정적인 HomeKit AID를 `devices.json`에 저장하는
  레지스트리 이관 기능.
- 새로운 BLE 제품을 독립적으로 확장할 수 있는 제품 메타데이터 레지스트리.
- BLE `frmCnt` 기반 중복·역순 프레임 제거와 10분 무수신 후 기준 초기화.
- Go 포맷·테스트·vet, Python 설치 테스트, ShellCheck, MIPSLE 빌드와 바이너리
  크기·SHA-256 검사를 수행하는 GitHub Actions 검증.

### 변경

- BLE 파서가 `gwts`를 보존하고 EID 데이터를 제품별 디코더로 전달하며, 구성된
  제품을 PDID 또는 알려진 모델로 해석하도록 변경.
- 기존 PDID 5860 이관과 안정적인 AID는 유지하면서 검색, 레지스트리, 시작 로그와
  런타임 매칭을 특정 센서에 종속되지 않는 BLE 장치 구조로 변경.
- 실행 중 발견된 지원 장치를 레지스트리에 저장하고 다음 브리지 재시작 때
  HomeKit에 노출하도록 자동 등록 동작 개선.
- 기존 장치의 해시 기반 AID를 최초 레지스트리 이관 시 그대로 보존하여 기존
  HomeKit 페어링과 자동화의 액세서리 ID가 바뀌지 않도록 개선.
- MQTT 구독에 `openmiio/report`를 추가하고 업데이트 큐를 확대했으며, 큐 포화
  로그를 제한된 빈도로 기록하도록 변경.
- 고정된 부팅 지연 대신 MQTT 1883 포트 준비 상태를 확인한 뒤 브리지를 시작.
- 새 센서 검색 시간을 고려하여 무텔넷 설치 후 HomeKit 포트 확인 시간을 60초로
  확대.
- 한국어·영어 README, 설치 및 프로토콜 문서를 새로운 등록 정책에 맞게
  갱신.

### 보안

- 기존 구성에서는 HomeKit 페어링 PIN을 매 시작 로그에 다시 기록하지 않으며,
  새 구성을 만든 최초 실행에만 표시.
- 브리지와 openmiio 로그를 권한 `600`으로 생성하고 크기 제한 로그 순환 적용.
- 무텔넷 설치 번들의 무결성 검증을 SHA-256 우선 방식으로 변경. `sha256sum`이
  없는 구형 BusyBox에서만 기존 MD5를 호환 대체 수단으로 사용.
- 공식 `openmiio_agent` v1.2.1 MIPS 바이너리를 SHA-256과 MD5로 모두 검증.
- PID 파일의 숫자뿐 아니라 `/proc/<pid>/cmdline`을 확인하여 PID 재사용으로
  엉뚱한 프로세스를 중지하거나 정상으로 판단하지 않도록 강화.

### 수정

- MGL03 재부팅 직후 새 BLE 보고가 도착하기 전까지 온도 센서가 `0°C`로
  표시되던 문제. 마지막 온도·습도·배터리 측정값을 권한 `600`의
  `measurements.json`에 원자적으로 저장하고 HomeKit 시작 전에 복원합니다.
  칫솔 동작과 같은 일시적 상태는 저장하지 않습니다.
- 실제 T700i 강제·조기 종료가 이전 완료 세션의 timestamp를 재사용하는 문제.
  활성 세션은 현재 게이트웨이 수신 시각을 실제 종료로 사용하여 watchdog을
  기다리지 않고 즉시 비활성으로 전환합니다.
- 시작 직후 또는 짧은 MQTT 재연결 뒤 연결된 센서 전체가 `응답 없음`으로 표시되던
  문제. 서비스별 선택 사항인 활성·오류 특성을 제거하여 HomeKit이 브리지 연결을
  기준으로 상태를 판단하고 일시적인 MQTT 공백에는 마지막 값을 유지하도록 했습니다.
- 동일한 BLE 보고가 `central/report`와 `miio/report`에 모두 도착할 때 HomeKit
  값을 중복 갱신하던 문제.
- MIoT 온도·습도·배터리 대체 경로에서 범위를 벗어난 값을 허용하던 문제.
- 필수 `miio central mqtt cache` 인수 없이 실행된 `openmiio_agent`를 정상
  프로세스로 오인할 수 있던 시작 스크립트 문제.
- 종료 스크립트가 브리지의 정상 종료를 기다리지 않고 PID 파일을 제거하던 문제.
- Linux amd64에서 `syscall.SO_REUSEPORT`가 정의되지 않아 GitHub Actions 빌드가
  실패하던 문제. 아키텍처별 값을 제공하는 `x/sys/unix`를 사용하도록 변경.
- 기본 MGL03 BusyBox에 `pidof`가 없어 기존 openmiio 프로세스를 감지하지 못하던
  문제. `/proc/[0-9]*/cmdline`을 직접 검사하도록 변경.

## 초기 개발 - 2026-08-10 ~ 2026-08-11

### 추가

- Xiaomi Gateway 3(`lumi.gateway.mgl03`)에서 실행되는 MIPSLE/soft-float
  HomeKit BLE 브리지.
- `miaomiaoce.sensor_ht.o2` / LYWSD02MMC(PDID 5860)의 온도, 습도, 배터리 및
  배터리 부족 상태 지원.
- `central/report`와 `miio/report`를 읽는 경량 MQTT 3.1.1 클라이언트.
- HomeKit 페어링 및 센서 식별 정보의 영구 저장과 다중 센서 구성.
- MGL03 기본 HomeKit 서비스와 UDP 5353을 공유하기 위한 dnssd 주소 재사용 패치.
- 저메모리 Go 빌드, 시작·중지·자동 시작·정리 스크립트.
- 펌웨어 1.5.0~1.5.4용 무텔넷 설치, 체크섬 검증 및 실패 시 자동 롤백.
- 한국어·영어 README, 설치 문서 및 프로토콜 문서.
