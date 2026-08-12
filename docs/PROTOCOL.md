# `miaomiaoce.sensor_ht.o2` 프로토콜 매핑

[English](PROTOCOL.en.md)

현재 릴리스는 한 가지 Xiaomi BLE 제품군을 지원합니다. 제품 메타데이터는
내부 레지스트리로 분리되어 이후 장치 디코더를 추가할 수 있습니다.

| 필드 | 값 |
|---|---|
| Xiaomi 모델 | `miaomiaoce.sensor_ht.o2` |
| 시판 모델 | `LYWSD02MMC` |
| 제품 ID(`pdid`) | `5860` |
| 전송 형식 | MiBeacon v2 이벤트 또는 MIoT 속성 보고 |

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

`pdid=5860`인 BLE 이벤트만 허용합니다. MIoT 속성 보고에는 제품 ID가 없으므로
해당 `did`가 이미 구성되었거나 검색된 센서와 일치할 때만 적용합니다.

브리지는 MAC/DID별 `frmCnt`를 추적합니다. 같은 카운터로 두 MQTT 토픽에
게시된 중복 프레임과 명백히 이전인 프레임은 무시하며, 10분 이상 보고가 없으면
센서 재부팅이나 배터리 교체를 허용하도록 카운터 기준을 초기화합니다. MQTT가
잠시 끊기면 마지막으로 확인된 HomeKit 값을 유지하면서 백그라운드에서
재연결합니다. 브리지 연결 자체가 끊긴 경우에는 HomeKit이 응답 없음을 판정합니다.

## 참고 자료

- [XiaomiGateway3 장치 데이터베이스](https://github.com/AlexxIT/XiaomiGateway3/blob/master/custom_components/xiaomi_gateway3/core/devices.py)
- [XiaomiGateway3 BLE 이벤트 처리기](https://github.com/AlexxIT/XiaomiGateway3/blob/master/custom_components/xiaomi_gateway3/core/gate/ble.py)
- [openmiio_agent central 프록시](https://github.com/AlexxIT/openmiio_agent/blob/master/internal/central/init.go)
