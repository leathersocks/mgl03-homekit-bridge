# 지원하는 Xiaomi BLE 프로토콜 매핑

[English](PROTOCOL.en.md)

제품 메타데이터와 BLE 디코더를 내부 레지스트리로 분리하여 런타임 곳곳에
PDID 조건문을 추가하지 않고도 새로운 장치 제품군을 확장할 수 있습니다.

| Xiaomi 모델 | 시판 모델 | PDID | HomeKit 서비스 |
|---|---|---:|---|
| `miaomiaoce.sensor_ht.o2` | LYWSD02MMC | `5860` | 온도, 습도, 배터리 |
| `k0918.toothbrush.t700i` | MES604 / T700i | `6032` | 동작(양치), 배터리 |

## openmiio 입력

`openmiio_agent miio central mqtt cache`는
`/tmp/central_service_lite.socket`의 메시지를 프록시하여 변경 없이 로컬 MQTT
토픽 `central/report`에 게시합니다.

일반적인 BLE 보고 형식은 다음과 같습니다.

```json
{
  "method": "_async.ble_event",
  "params": {
    "dev": {
      "did": "blt.3.example",
      "mac": "AA:BB:CC:DD:EE:FF",
      "pdid": 5860
    },
    "evt": [
      { "eid": 19457, "edata": "3333bb41" },
      { "eid": 19458, "edata": "2d" },
      { "eid": 18435, "edata": "58" }
    ],
    "frmCnt": 36
  }
}
```

## 측정값

| 측정값 | MiBeacon 이벤트 | 인코딩 | MIoT 대체 경로 |
|---|---:|---|---|
| 온도 | `19457` (`0x4C01`) | IEEE-754 float32, 리틀 엔디언, 0.1 °C 단위 반올림 | `siid=3`, `piid=1001` |
| 습도 | `19458` (`0x4C02`) | 부호 없는 바이트, 백분율 | `siid=3`, `piid=1002` |
| 배터리 | `18435` (`0x4803`) | 부호 없는 바이트, 백분율 | `siid=2`, `piid=1003` |

## T700i 양치 이벤트

T700i 상태 이벤트는 EID `12291`(`0x3003`)을 사용합니다. 페이로드는 이벤트
타입 1바이트, 리틀 엔디언 Unix timestamp 4바이트와 선택적인 점수 1바이트로
구성됩니다. 표준 배터리 보고는 EID `4106`(`0x100A`)을 사용합니다.

```text
시작:      00 9b 5d 77 6a       type=0, timestamp=1786207643
강제 종료: 01 b9 20 77 6a       type=1, 이전 세션의 오래된 timestamp
배터리:    64                   100퍼센트
```

`type=0`은 양치 세션을 시작하거나 갱신하고 0이 아닌 타입은 종료 후보입니다.
내장 timestamp가 `gwts`와 60초 이내이면 실시간 이벤트로 처리합니다. 실제
T700i 강제 종료 패킷은 이전 완료 세션의 timestamp를 재사용할 수 있으므로,
현재 세션이 활성 상태이고 10분 이내이면 `gwts`를 실제 종료 시각으로 사용합니다.
종료 광고가 유실되면 30초 activity watchdog이 HomeKit 동작 센서를 비활성으로
복구합니다. 점수와 양치 시간은 진단용으로 보존하지만 비표준 HomeKit 특성으로
노출하지 않습니다.

제품 레지스트리에 디코더가 등록된 PDID의 BLE 이벤트만 허용합니다. MIoT 속성
보고에는 제품 ID가 없으므로 해당 `did`가 이미 구성되었거나 검색된 장치와
일치할 때만 적용합니다.

브리지는 MAC/DID별 `frmCnt`를 추적합니다. 같은 카운터로 두 MQTT 토픽에
게시된 중복 프레임과 명백히 이전인 프레임은 무시하며, 10분 이상 보고가 없으면
센서 재부팅이나 배터리 교체를 허용하도록 카운터 기준을 초기화합니다. MQTT가
잠시 끊기면 마지막으로 확인된 HomeKit 값을 유지하면서 백그라운드에서
재연결합니다. 브리지 연결 자체가 끊긴 경우에는 HomeKit이 응답 없음을 판정합니다.

동일한 T700i 시작 프레임은 이미 활성화된 watchdog을 갱신할 수 있지만 최초
세션 시작 시각을 변경하지 않습니다. 역순의 오래된 프레임은 칫솔 상태를
변경하지 않습니다.

## 재시작 측정값 복원

브리지는 값이 실제로 변경될 때 마지막 온도·습도·배터리를
`/data/mgl03-homekit/measurements.json`에 원자적으로 저장합니다. MGL03가
재부팅되면 이 값을 HomeKit 서버 시작 전에 복원하고, 이후 새 BLE 보고로
교체합니다. 파일은 권한 `600`으로 생성됩니다. 오래된 동작 감지가 재부팅 후
활성화되지 않도록 T700i 세션과 동작 상태는 저장하지 않습니다.

## 참고 자료

- [XiaomiGateway3 장치 데이터베이스](https://github.com/AlexxIT/XiaomiGateway3/blob/master/custom_components/xiaomi_gateway3/core/devices.py)
- [XiaomiGateway3 BLE 이벤트 처리기](https://github.com/AlexxIT/XiaomiGateway3/blob/master/custom_components/xiaomi_gateway3/core/gate/ble.py)
- [openmiio_agent central 프록시](https://github.com/AlexxIT/openmiio_agent/blob/master/internal/central/init.go)
