# MGL03 설치

[English](INSTALL.en.md)

> 이 프로젝트는 커뮤니티 통합 기능으로 Apple 인증 제품이 아니며, 게이트웨이에서
> 실행되는 소프트웨어를 변경합니다. MGL03을 복구할 수단을 준비하고 openmiio
> MQTT 포트를 인터넷에 노출하지 마십시오.

## 요구 사항

- Xiaomi Gateway 3, 모델 `lumi.gateway.mgl03`.
- Telnet 없는 설치 프로그램을 사용하려면 펌웨어 `1.5.0`부터 `1.5.4`까지와
  해당 장치의 32자 miIO 토큰이 필요합니다. 다른 펌웨어에는 호환되는 수동 접속
  방법이 필요합니다.
- 같은 LAN의 PC에 Python 3 및 `python-miio==0.5.12`.
- `miaomiaoce.sensor_ht.o2` / LYWSD02MMC가 Bluetooth를 통해 게이트웨이에
  이미 표시되어 있어야 합니다.
- 최초 HomeKit 페어링을 위한 같은 LAN의 iPhone/iPad.

## Telnet 없이 설치

먼저 MIPSLE 브리지 바이너리를 빌드한 다음 PC 측 종속성을 설치합니다.

```powershell
Set-Location C:\Git\mgl03-homekit-bridge
.\scripts\build.ps1 -Version dev
py -m pip install -r .\requirements-installer.txt
```

설치 프로그램을 실행합니다. 토큰은 숨김 프롬프트에서 입력하며 번들, 명령줄,
로그 또는 게이트웨이 파일 시스템에 기록되지 않습니다.

```powershell
py .\scripts\install_no_telnet.py --gateway-ip 192.168.10.41
```

설치 프로그램은 Telnet 세션을 열지 않고 다음 작업을 수행합니다.

1. 로컬 miIO를 통해 모델과 지원 펌웨어를 확인합니다.
2. 공식 MIPS용 `openmiio_agent` v1.2.1을 다운로드하거나 검증합니다.
3. 바이너리와 스크립트만 포함한 임시 HTTP 서버를 시작합니다. 토큰, Gateway
   Key, HomeKit PIN 또는 페어링 상태는 절대 포함하지 않습니다.
4. 게이트웨이가 번들을 가져오도록 지시하는 짧은 `set_ip_info` 요청을
   보냅니다.
5. 게이트웨이에서 모든 파일의 SHA-256을 확인하고, 알 수 없는 시작 훅은
   거부하며, 런타임 파일을 원자적으로 교체한 뒤 브리지 실행에 실패하면
   롤백합니다. `sha256sum`이 없는 구형 BusyBox에서만 MD5로 대체합니다.
6. 일회성 HTTP 콜백으로 결과를 수신하고 브리지가 준비되면 TCP `51826`을
   확인합니다.

임시 서버는 사용 가능한 포트를 자동으로 선택합니다. 라우팅 또는 방화벽 정책상
필요할 때만 PC 주소나 포트를 지정하십시오.

```powershell
py .\scripts\install_no_telnet.py `
  --gateway-ip 192.168.10.41 `
  --pc-ip 192.168.10.100 `
  --http-port 8000
```

설치 중 PC에서 GitHub에 접속할 수 없다면 미리 다운로드한 공식 MIPS 바이너리를
지정하십시오. 예상 SHA-256은
`78c775b354bb5fb896682fd3c26b9114cf336387985629ca16bc40a19cfb74f6`입니다.

```powershell
py .\scripts\install_no_telnet.py `
  --gateway-ip 192.168.10.41 `
  --openmiio-bin C:\path\to\openmiio_agent_mips
```

이 경로는 `lumi.gateway.mgl03` 펌웨어 `1.5.0`-`1.5.4`로 제한됩니다. 다른
인증 명령을 사용하는 `1.5.5+`에서는 실행을 거부하며 펌웨어를 변경하지
않습니다. TCP 23번 포트는 닫힌 상태로 유지됩니다. 설치된 브리지를 업데이트할
때 기존 `/data/mgl03-homekit/config.json`, `devices.json`, `hap` 및 로그를
보존합니다.

## 수동 Telnet 대체 방법

설치된 펌웨어를 Telnet 없는 설치 프로그램이 지원하지 않거나 설치 실패를 진단할
때만 사용하십시오. 해당 펌웨어에 맞는 방법으로 임시 셸 접속을 활성화한 다음,
동일한 런타임 파일을 수동으로 복사합니다.

### 파일 복사

컴퓨터에서 프로젝트를 빌드한 다음 다음 파일을 게이트웨이에 복사합니다.

| 로컬 파일 | MGL03 경로 |
|---|---|
| `bin/mgl03-homekit-bridge` | `/data/mgl03-homekit-bridge` |
| `scripts/start.sh` | `/data/mgl03-homekit-start.sh` |
| `scripts/stop.sh` | `/data/mgl03-homekit-stop.sh` |
| `scripts/startup.sh` | `/data/scripts/startup.sh` |
| `scripts/cleanup.sh` | `/data/mgl03-homekit-cleanup.sh` |

게이트웨이에서 실행합니다.

```sh
mkdir -p /data/scripts
chmod 755 /data/openmiio_agent /data/mgl03-homekit-bridge
chmod 755 /data/mgl03-homekit-start.sh /data/mgl03-homekit-stop.sh
chmod 755 /data/scripts/startup.sh
chmod 755 /data/mgl03-homekit-cleanup.sh
/data/mgl03-homekit-start.sh
tail -f /data/mgl03-homekit/bridge.log
```

초기 MGL03은 사용 가능한 RAM이 약 58 MiB뿐이며 스왑이 없습니다. 따라서 제공된
시작 스크립트는 OS 스레드 1개, 16 MiB Go 메모리 제한 및 더 빈번한 가비지
컬렉션 설정으로 브리지를 실행합니다. 스크립트를 호출하기 전에 `GOMAXPROCS`,
`GOMEMLIMIT` 또는 `GOGC`를 설정하여 기본값을 변경할 수 있습니다.

스크립트는 `pidof`가 없는 기본 BusyBox에서도 `/proc/[0-9]*/cmdline`을 직접
확인하여 필수 `miio central mqtt cache` 인수 없이 실행된 `openmiio_agent`를
정상 프로세스로 오인하지 않습니다. 두 데몬 중 하나라도 시작 중 종료되면 실패를
반환합니다.

첫 시작 시 무작위 페어링 PIN이 포함된 `/data/mgl03-homekit/config.json`을
만든 다음 30초 검색 창에서 `miaomiaoce.sensor_ht.o2` 광고를 수집합니다.
페어링 코드는 구성을 만든 최초 실행에만 권한이 제한된 로그에 표시됩니다. Apple 홈에서
**액세서리 추가 → 추가 옵션**을 선택하고 **MGL03 Bluetooth Bridge**를 고른
다음 해당 코드를 입력하십시오.

검색된 MAC, DID, 제품 ID와 안정적인 HomeKit AID는
`/data/mgl03-homekit/devices.json`에 저장됩니다. 페어링
키는 `/data/mgl03-homekit/hap`에 보관되므로 재부팅과 업그레이드 시 두 위치를
모두 보존하십시오.

## 여러 센서 사용

새 구성의 `discovery.mode` 기본값은 `auto`입니다. 시작 시 검색 창에서 여러
센서를 등록하며, 실행 중 새 센서를 발견하면 `devices.json`에 저장합니다. 실행
중 추가된 센서는 브리지를 한 번 재시작하면 HomeKit에 나타납니다. 기존 구성은
호환성을 위해 `first` 방식으로 유지됩니다. 자동 등록을 원하지 않으면
`discovery.mode`를 `manual`로 설정하십시오.

## 자동 시작

기본 `1.5.0_0026` 펌웨어는 읽기 전용 SquashFS 루트를 사용하지만
`/etc/init.d/rcS`에서 `/data/scripts/startup.sh`를 확인합니다. 이 사용자 지정
파일이 실행 가능하면 일반 `/bin/startup.sh` 명령을 대신합니다. 제공된 래퍼는
먼저 브리지 실행을 예약하고, 일반 게이트웨이 서비스를 위해 30초 기다린 다음,
기본 명령으로 제어를 넘깁니다. `/bin/startup.sh`는 반환하지 않고 계속 실행되므로
이 순서가 중요합니다. HomeKit 시작 출력은
`/data/mgl03-homekit/startup.log`에 기록됩니다.

`/etc/init.d/rcS`를 수정하지 마십시오. 대신 쓰기 가능한 `/data` 파티션에
래퍼를 복사합니다.

```sh
mkdir -p /data/scripts
chmod 755 /data/scripts/startup.sh
```

`/data/scripts/startup.sh`가 없을 때만 제공된 래퍼를 설치하십시오. 해당 경로를
다른 사용자 지정 기능이 이미 사용 중이라면 기존 기본 시작 처리를 보존하고,
덮어쓰는 대신 다음 비동기 호출만 병합하십시오.

```sh
(
    sleep 30
    /data/mgl03-homekit-start.sh
) >>/data/mgl03-homekit/startup.log 2>&1 &
```

재부팅 후 약 1분 기다린 다음 부팅 래퍼와 브리지를 모두 확인합니다.

```sh
cat /data/mgl03-homekit/startup.log
cat /data/mgl03-homekit/bridge.log
ps | grep '[m]gl03-homekit-bridge'
netstat -lnt | grep 51826
```

펌웨어 업데이트로 부팅 구성이 바뀌거나 셸 접속이 비활성화될 수 있습니다.
업데이트 후에는 이 훅에 의존하기 전에 다시 확인하십시오.

## 설치 잔여 파일 제거

정리 스크립트에는 MGL03 초기 구성 과정에서 생성된 폐기된 테스트 바이너리,
스테이징 파일 및 이전 페어링 백업만 정확히 지정한 허용 목록이 있습니다. 현재
브리지, 시작 스크립트, 구성, 장치 레지스트리 또는 HomeKit 상태가 없으면 실행을
거부합니다. 첫 실행은 항상 모의 실행입니다.

```sh
/data/mgl03-homekit-cleanup.sh
```

보고된 경로를 검토한 다음 정리를 적용합니다.

```sh
/data/mgl03-homekit-cleanup.sh --apply
```

여러 센서를 등록하기 전의 장치 레지스트리 백업은 기본적으로 유지됩니다. 구성된
모든 센서가 작동하는지 확인한 뒤에만 제거하십시오.

```sh
/data/mgl03-homekit-cleanup.sh --apply --include-recovery
```

현재 바이너리, 페어링 상태, 센서 3개 레지스트리, PID 파일 및 사용 중인
브리지/openmiio/시작 로그는 정리 대상이 아닙니다.

## HomeKit 페어링 초기화

브리지를 중지하고 백업한 다음 `/data/mgl03-homekit/hap`만 삭제하십시오.
브리지를 시작하고 다시 페어링합니다. 센서 검색도 초기화해야 하는 경우가 아니면
`devices.json`을 유지하십시오.
